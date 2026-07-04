package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Lucy-97/browser-agent/backend-api/internal/config"
	"github.com/Lucy-97/browser-agent/backend-api/internal/database"
	automationengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/automation"
	workerengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/worker"
	basehandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler"
	automationhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/automation"
	workerhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/worker"
	"github.com/Lucy-97/browser-agent/backend-api/internal/lock"
	automationrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/automation"
	workerrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/worker"
)

type Server struct {
	handler http.Handler
}

func NewServer() *Server {
	return NewServerWithConfig(config.Load())
}

func NewServerWithConfig(cfg config.Config) *Server {
	mux := http.NewServeMux()

	workerRepo, automationRepo := buildRepositories(cfg)
	claimLocker := buildClaimLocker(cfg)

	workerEngine := workerengine.New(workerRepo)
	automationEngine := automationengine.New(automationRepo, claimLocker)

	workerHandler := workerhandler.New(workerEngine)
	automationHandler := automationhandler.New(automationEngine, workerHandler, cfg.ArtifactDir)

	workerHandler.Register(mux)
	automationHandler.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return &Server{handler: withLocalCORS(withRoleAuth(mux, cfg))}
}

func buildRepositories(cfg config.Config) (workerengine.Repository, automationengine.Repository) {
	if cfg.MySQLDSN == "" {
		return workerrepo.NewMemoryRepository(), automationrepo.NewMemoryRepository()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := database.OpenMySQL(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return workerrepo.NewMySQLRepository(db), automationrepo.NewMySQLRepository(db)
}

func (server *Server) Handler() http.Handler {
	return server.handler
}

func buildClaimLocker(cfg config.Config) lock.Locker {
	if cfg.RedisAddr == "" {
		return lock.NoopLocker{}
	}
	return lock.NewRedisLocker(lock.RedisConfig{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		Timeout:  2 * time.Second,
	})
}

func withLocalCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Secret, X-Admin-Token, X-Web-Token")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")
}

func withRoleAuth(next http.Handler, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/admin/") && cfg.AdminAPIToken != "":
			if !tokenMatches(r, "X-Admin-Token", cfg.AdminAPIToken) {
				basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid admin token", false)
				return
			}
		case strings.HasPrefix(r.URL.Path, "/web/") && cfg.WebAPIToken != "":
			if !tokenMatches(r, "X-Web-Token", cfg.WebAPIToken) {
				basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid web token", false)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func tokenMatches(r *http.Request, headerName string, expected string) bool {
	token := strings.TrimSpace(r.Header.Get(headerName))
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	return token != "" && token == expected
}
