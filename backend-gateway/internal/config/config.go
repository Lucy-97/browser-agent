package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 网关全部配置。
type Config struct {
	Server   ServerConfig
	JWT      JWTConfig
	Redis    RedisConfig
	Services ServiceRoutes
	Internal InternalConfig
	CORS     CORSConfig
}

type ServerConfig struct {
	Port string
}

type JWTConfig struct {
	Secret              string
	AccessTokenExpSec   int
	RefreshTokenExpDays int
	PublicPaths         []string
	PublicPathPrefixes  []string
	AccessTokenCookie   string
}

type RedisConfig struct {
	Host          string
	Port          string
	Password      string
	DB            int
	Required      bool
	TLSEnabled    bool
	TLSServerName string
	TLSCAFile     string
}

// ServiceRoutes 定义下游服务地址。
type ServiceRoutes struct {
	APIService      string
	AIEngineService string
}

type InternalConfig struct {
	Secret string
}

type CORSConfig struct {
	AllowedOrigins []string
}

// Load 从环境变量读取配置。
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("GATEWAY_PORT", "8080"),
		},
		JWT: JWTConfig{
			Secret:              getEnvSecret("JWT_SECRET"),
			AccessTokenExpSec:   getEnvInt("JWT_ACCESS_TOKEN_EXP_SEC", 3600),
			RefreshTokenExpDays: getEnvInt("JWT_REFRESH_TOKEN_EXP_DAYS", 30),
			PublicPaths: []string{
				"/health",
				"/api/v1/auth/register",
				"/api/v1/auth/login",
				"/api/v1/auth/logout",
			},
			PublicPathPrefixes: []string{
				"/oauth2/",
				"/login/oauth2/",
				"/worker/",
			},
			AccessTokenCookie: getEnv("AUTH_COOKIE_NAME", "browser_agent_access"),
		},
		Redis: RedisConfig{
			Host:          getEnv("REDIS_HOST", "localhost"),
			Port:          getEnv("REDIS_PORT", "6379"),
			Password:      getEnvOrFile("REDIS_PASSWORD", ""),
			DB:            getEnvInt("REDIS_DB", 0),
			Required:      getEnvBool("REDIS_REQUIRED", false),
			TLSEnabled:    getEnvBool("REDIS_TLS_ENABLED", false),
			TLSServerName: getEnv("REDIS_TLS_SERVER_NAME", ""),
			TLSCAFile:     getEnv("REDIS_TLS_CA_FILE", ""),
		},
		Services: ServiceRoutes{
			APIService:      getEnv("API_SERVICE_URL", "http://localhost:8001"),
			AIEngineService: getEnv("AI_ENGINE_SERVICE_URL", "http://localhost:8002"),
		},
		Internal: InternalConfig{Secret: getEnvOrFile("INTERNAL_API_SECRET", "")},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ORIGINS", "http://localhost:3000,http://localhost:5174"),
		},
	}
}

func (cfg RedisConfig) TLSConfig() (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil
	}
	serverName := strings.TrimSpace(cfg.TLSServerName)
	if serverName == "" {
		serverName = strings.TrimSpace(cfg.Host)
	}
	if serverName == "" {
		return nil, fmt.Errorf("REDIS_TLS_SERVER_NAME or REDIS_HOST is required when Redis TLS is enabled")
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	if cfg.TLSCAFile == "" {
		return tlsConfig, nil
	}
	caPEM, err := os.ReadFile(cfg.TLSCAFile)
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSecret(key string) string {
	v := strings.TrimSpace(getEnvOrFile(key, ""))
	if len(v) < 32 {
		if v == "" {
			panic("FATAL: Missing required secret: " + key)
		}
		panic("FATAL: Secret must contain at least 32 characters: " + key)
	}
	return v
}

func getEnvOrFile(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	path := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if path == "" {
		return fallback
	}
	value, err := os.ReadFile(path)
	if err != nil {
		panic("failed to read " + key + "_FILE: " + err.Error())
	}
	return strings.TrimSpace(string(value))
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvSlice(key, fallback string) []string {
	v := os.Getenv(key)
	if v == "" {
		v = fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
