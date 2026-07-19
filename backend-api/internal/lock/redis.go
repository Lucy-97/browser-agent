package lock

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type RedisConfig struct {
	Addr      string
	Password  string
	DB        int
	Timeout   time.Duration
	TLSConfig *tls.Config
}

type RedisLocker struct {
	config RedisConfig
}

func NewRedisLocker(config RedisConfig) *RedisLocker {
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Second
	}
	return &RedisLocker{config: config}
}

func (locker *RedisLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error) {
	token := randomToken()
	conn, reader, err := locker.dial(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()

	if err := writeCommand(conn, "SET", key, token, "NX", "PX", strconv.FormatInt(ttl.Milliseconds(), 10)); err != nil {
		return nil, false, err
	}
	reply, err := readSimpleReply(reader)
	if err != nil {
		return nil, false, err
	}
	if reply == "" {
		return nil, false, nil
	}
	if reply != "OK" {
		return nil, false, fmt.Errorf("unexpected redis SET reply: %s", reply)
	}
	return redisLock{locker: locker, key: key, token: token}, true, nil
}

func (locker *RedisLocker) dial(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	dialer := net.Dialer{Timeout: locker.config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", locker.config.Addr)
	if err != nil {
		return nil, nil, err
	}
	if locker.config.TLSConfig != nil {
		tlsConn := tls.Client(conn, locker.config.TLSConfig.Clone())
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("Redis TLS handshake: %w", err)
		}
		conn = tlsConn
	}
	reader := bufio.NewReader(conn)

	if locker.config.Password != "" {
		if err := writeCommand(conn, "AUTH", locker.config.Password); err != nil {
			conn.Close()
			return nil, nil, err
		}
		if _, err := readSimpleReply(reader); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	if locker.config.DB > 0 {
		if err := writeCommand(conn, "SELECT", strconv.Itoa(locker.config.DB)); err != nil {
			conn.Close()
			return nil, nil, err
		}
		if _, err := readSimpleReply(reader); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	return conn, reader, nil
}

type redisLock struct {
	locker *RedisLocker
	key    string
	token  string
}

func (lock redisLock) Release(ctx context.Context) error {
	conn, reader, err := lock.locker.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	script := "if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) else return 0 end"
	if err := writeCommand(conn, "EVAL", script, "1", lock.key, lock.token); err != nil {
		return err
	}
	_, err = readIntegerReply(reader)
	return err
}

func writeCommand(conn net.Conn, args ...string) error {
	if _, err := fmt.Fprintf(conn, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

func readSimpleReply(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if line == "$-1" {
		return "", nil
	}
	switch {
	case strings.HasPrefix(line, "+"):
		return strings.TrimPrefix(line, "+"), nil
	case strings.HasPrefix(line, "$"):
		length, err := strconv.Atoi(strings.TrimPrefix(line, "$"))
		if err != nil {
			return "", err
		}
		if length < 0 {
			return "", nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return string(buf[:length]), nil
	case strings.HasPrefix(line, "-"):
		return "", errors.New(strings.TrimPrefix(line, "-"))
	default:
		return "", fmt.Errorf("unexpected redis reply: %s", line)
	}
}

func readIntegerReply(reader *bufio.Reader) (int64, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if strings.HasPrefix(line, "-") {
		return 0, errors.New(strings.TrimPrefix(line, "-"))
	}
	if !strings.HasPrefix(line, ":") {
		return 0, fmt.Errorf("unexpected redis integer reply: %s", line)
	}
	return strconv.ParseInt(strings.TrimPrefix(line, ":"), 10, 64)
}

func randomToken() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}
