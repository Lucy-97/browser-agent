package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	Addr                   string
	ArtifactStore          string
	ArtifactDir            string
	R2AccountID            string
	R2Bucket               string
	R2AccessKeyID          string
	R2SecretAccessKey      string
	R2Prefix               string
	InternalSecret         string
	AdminAPIToken          string
	WebAPIToken            string
	DefaultTenantID        string
	DefaultUserID          string
	RequireTenantIdentity  bool
	RequireMembership      bool
	RequirePairingApproval bool
	JWTSecret              string
	JWTAccessTokenExpSec   int
	AuthCookieName         string
	AuthCookieSecure       bool
	AllowRegistration      bool
	MySQLDSN               string
	MySQLMaxOpen           int
	MySQLMaxIdle           int
	RedisAddr              string
	RedisPassword          string
	RedisDB                int
	RedisTLSEnabled        bool
	RedisTLSServerName     string
	RedisTLSCAFile         string
	AdminAPIURL            string
}

func Load() Config {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":29001"
	}
	return Config{
		Addr:                   addr,
		ArtifactStore:          envString("ARTIFACT_STORE", "local"),
		ArtifactDir:            os.Getenv("ARTIFACT_DIR"),
		R2AccountID:            os.Getenv("R2_ACCOUNT_ID"),
		R2Bucket:               os.Getenv("R2_BUCKET"),
		R2AccessKeyID:          envOrFile("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:      envOrFile("R2_SECRET_ACCESS_KEY"),
		R2Prefix:               envString("R2_PREFIX", "artifacts"),
		InternalSecret:         envOrFile("INTERNAL_SECRET"),
		AdminAPIToken:          os.Getenv("ADMIN_API_TOKEN"),
		WebAPIToken:            os.Getenv("WEB_API_TOKEN"),
		DefaultTenantID:        envString("DEFAULT_TENANT_ID", "tenant_local"),
		DefaultUserID:          envString("DEFAULT_USER_ID", "user_local"),
		RequireTenantIdentity:  envBool("REQUIRE_TENANT_IDENTITY", false),
		RequireMembership:      envBool("REQUIRE_MEMBERSHIP_VALIDATION", false),
		RequirePairingApproval: envBool("REQUIRE_WORKER_PAIRING_APPROVAL", false),
		JWTSecret:              envOrFile("JWT_SECRET"),
		JWTAccessTokenExpSec:   envInt("JWT_ACCESS_TOKEN_EXP_SEC", 3600),
		AuthCookieName:         envString("AUTH_COOKIE_NAME", "browser_agent_access"),
		AuthCookieSecure:       envBool("AUTH_COOKIE_SECURE", false),
		AllowRegistration:      envBool("ALLOW_PUBLIC_REGISTRATION", false),
		MySQLDSN:               envOrFile("MYSQL_DSN"),
		MySQLMaxOpen:           envInt("MYSQL_MAX_OPEN_CONNS", 10),
		MySQLMaxIdle:           envInt("MYSQL_MAX_IDLE_CONNS", 5),
		RedisAddr:              os.Getenv("REDIS_ADDR"),
		RedisPassword:          envOrFile("REDIS_PASSWORD"),
		RedisDB:                envInt("REDIS_DB", 0),
		RedisTLSEnabled:        envBool("REDIS_TLS_ENABLED", false),
		RedisTLSServerName:     os.Getenv("REDIS_TLS_SERVER_NAME"),
		RedisTLSCAFile:         os.Getenv("REDIS_TLS_CA_FILE"),
		AdminAPIURL:            os.Getenv("ADMIN_API_BASE_URL"),
	}
}

func (cfg Config) RedisTLSConfig() (*tls.Config, error) {
	if !cfg.RedisTLSEnabled {
		return nil, nil
	}
	serverName := strings.TrimSpace(cfg.RedisTLSServerName)
	if serverName == "" {
		host, _, err := net.SplitHostPort(cfg.RedisAddr)
		if err != nil {
			return nil, fmt.Errorf("derive Redis TLS server name from REDIS_ADDR: %w", err)
		}
		serverName = host
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	if cfg.RedisTLSCAFile == "" {
		return tlsConfig, nil
	}
	caPEM, err := os.ReadFile(cfg.RedisTLSCAFile)
	if err != nil {
		return nil, fmt.Errorf("read REDIS_TLS_CA_FILE: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate pool: %w", err)
	}
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("REDIS_TLS_CA_FILE contains no valid certificates")
	}
	tlsConfig.RootCAs = roots
	return tlsConfig, nil
}

func envOrFile(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	path := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if path == "" {
		return ""
	}
	value, err := os.ReadFile(path)
	if err != nil {
		panic("failed to read " + key + "_FILE: " + err.Error())
	}
	return strings.TrimSpace(string(value))
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch os.Getenv(key) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n
}
