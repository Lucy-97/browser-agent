package config

import "os"

type Config struct {
	Addr                  string
	ArtifactDir           string
	InternalSecret        string
	AdminAPIToken         string
	WebAPIToken           string
	DefaultTenantID       string
	DefaultUserID         string
	RequireTenantIdentity bool
	MySQLDSN              string
	MySQLMaxOpen          int
	MySQLMaxIdle          int
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	AdminAPIURL           string
}

func Load() Config {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":29001"
	}
	return Config{
		Addr:                  addr,
		ArtifactDir:           os.Getenv("ARTIFACT_DIR"),
		InternalSecret:        os.Getenv("INTERNAL_SECRET"),
		AdminAPIToken:         os.Getenv("ADMIN_API_TOKEN"),
		WebAPIToken:           os.Getenv("WEB_API_TOKEN"),
		DefaultTenantID:       envString("DEFAULT_TENANT_ID", "tenant_local"),
		DefaultUserID:         envString("DEFAULT_USER_ID", "user_local"),
		RequireTenantIdentity: envBool("REQUIRE_TENANT_IDENTITY", false),
		MySQLDSN:              os.Getenv("MYSQL_DSN"),
		MySQLMaxOpen:          envInt("MYSQL_MAX_OPEN_CONNS", 10),
		MySQLMaxIdle:          envInt("MYSQL_MAX_IDLE_CONNS", 5),
		RedisAddr:             os.Getenv("REDIS_ADDR"),
		RedisPassword:         os.Getenv("REDIS_PASSWORD"),
		RedisDB:               envInt("REDIS_DB", 0),
		AdminAPIURL:           os.Getenv("ADMIN_API_BASE_URL"),
	}
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
