package config

import "os"

type Config struct {
	Addr           string
	ArtifactDir    string
	InternalSecret string
	AdminAPIToken  string
	WebAPIToken    string
	MySQLDSN       string
	MySQLMaxOpen   int
	MySQLMaxIdle   int
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	AdminAPIURL    string
}

func Load() Config {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":28001"
	}
	return Config{
		Addr:           addr,
		ArtifactDir:    os.Getenv("ARTIFACT_DIR"),
		InternalSecret: os.Getenv("INTERNAL_SECRET"),
		AdminAPIToken:  os.Getenv("ADMIN_API_TOKEN"),
		WebAPIToken:    os.Getenv("WEB_API_TOKEN"),
		MySQLDSN:       os.Getenv("MYSQL_DSN"),
		MySQLMaxOpen:   envInt("MYSQL_MAX_OPEN_CONNS", 10),
		MySQLMaxIdle:   envInt("MYSQL_MAX_IDLE_CONNS", 5),
		RedisAddr:      os.Getenv("REDIS_ADDR"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisDB:        envInt("REDIS_DB", 0),
		AdminAPIURL:    os.Getenv("ADMIN_API_BASE_URL"),
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
