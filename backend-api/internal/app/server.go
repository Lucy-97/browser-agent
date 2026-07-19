package app

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/Lucy-97/browser-agent/backend-api/internal/config"
	"github.com/Lucy-97/browser-agent/backend-api/internal/database"
	authengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/auth"
	automationengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/automation"
	workerengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/worker"
	basehandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler"
	authhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/auth"
	automationhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/automation"
	workerhandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler/worker"
	"github.com/Lucy-97/browser-agent/backend-api/internal/identity"
	"github.com/Lucy-97/browser-agent/backend-api/internal/lock"
	authrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/auth"
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

	workerRepo, automationRepo, authRepo := buildRepositories(cfg)
	claimLocker := buildClaimLocker(cfg)

	workerEngine := workerengine.New(workerRepo, workerengine.Options{
		AutoApprovePairing: !cfg.RequireTenantIdentity && !cfg.RequirePairingApproval,
		DefaultTenantID:    defaultString(cfg.DefaultTenantID, "tenant_local"),
		DefaultUserID:      defaultString(cfg.DefaultUserID, "user_local"),
	})
	automationEngine := automationengine.New(automationRepo, claimLocker)
	authEngine := authengine.New(authRepo, authengine.Options{
		JWTSecret:               cfg.JWTSecret,
		AccessTokenTTL:          time.Duration(cfg.JWTAccessTokenExpSec) * time.Second,
		AllowPublicRegistration: cfg.AllowRegistration,
	})

	workerHandler := workerhandler.New(workerEngine)
	automationHandler := automationhandler.New(automationEngine, workerHandler, cfg.ArtifactDir)
	authHandler := authhandler.New(authEngine, authhandler.Options{
		CookieName: cfg.AuthCookieName, CookieSecure: cfg.AuthCookieSecure,
	})

	authHandler.Register(mux)
	workerHandler.Register(mux)
	automationHandler.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return &Server{handler: withLocalCORS(withRoleAuth(withActorIdentity(mux, cfg, authEngine), cfg))}
}

func buildRepositories(cfg config.Config) (workerengine.Repository, automationengine.Repository, authengine.Repository) {
	if cfg.MySQLDSN == "" {
		return workerrepo.NewMemoryRepository(), automationrepo.NewMemoryRepository(), authrepo.NewMemoryRepository()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := database.OpenMySQL(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return workerrepo.NewMySQLRepository(db), automationrepo.NewMySQLRepository(db), authrepo.NewMySQLRepository(db)
}

func (server *Server) Handler() http.Handler {
	return server.handler
}

func buildClaimLocker(cfg config.Config) lock.Locker {
	if cfg.RedisAddr == "" {
		return lock.NoopLocker{}
	}
	tlsConfig, err := cfg.RedisTLSConfig()
	if err != nil {
		panic(err)
	}
	return lock.NewRedisLocker(lock.RedisConfig{
		Addr:      cfg.RedisAddr,
		Password:  cfg.RedisPassword,
		DB:        cfg.RedisDB,
		Timeout:   2 * time.Second,
		TLSConfig: tlsConfig,
	})
}

func withLocalCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
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

type membershipVerifier interface {
	IsActiveMembership(ctx context.Context, tenantID string, userID string, role string) (bool, error)
}

func withActorIdentity(next http.Handler, cfg config.Config, verifier membershipVerifier) http.Handler {
	defaultTenantID := defaultString(cfg.DefaultTenantID, "tenant_local")
	defaultUserID := defaultString(cfg.DefaultUserID, "user_local")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/web/") && !strings.HasPrefix(r.URL.Path, "/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		actor := identity.FromHeaders(r)
		if actor.Valid() {
			if cfg.InternalSecret == "" || !constantTimeHeaderMatches(r, "X-Internal-Secret", cfg.InternalSecret) {
				basehandler.WriteError(w, http.StatusForbidden, "UNTRUSTED_IDENTITY", "tenant identity must be supplied by the trusted gateway", false)
				return
			}
			if !actor.HasKnownRole() {
				basehandler.WriteError(w, http.StatusForbidden, "INVALID_TENANT_ROLE", "tenant role is missing or invalid", false)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/admin/") && actor.Role != identity.RolePlatformAdmin {
				basehandler.WriteError(w, http.StatusForbidden, "PLATFORM_ADMIN_REQUIRED", "platform admin role is required", false)
				return
			}
			if cfg.RequireMembership && strings.HasPrefix(r.URL.Path, "/web/") && actor.Role != identity.RolePlatformAdmin {
				active, err := verifier.IsActiveMembership(r.Context(), actor.TenantID, actor.UserID, actor.Role)
				if err != nil {
					basehandler.WriteError(w, http.StatusServiceUnavailable, "MEMBERSHIP_CHECK_FAILED", "tenant membership could not be verified", true)
					return
				}
				if !active {
					basehandler.WriteError(w, http.StatusForbidden, "MEMBERSHIP_INACTIVE", "active tenant membership is required", false)
					return
				}
			}
		} else {
			if cfg.RequireTenantIdentity {
				basehandler.WriteError(w, http.StatusUnauthorized, "TENANT_IDENTITY_REQUIRED", "authenticated tenant identity is required", false)
				return
			}
			actor = identity.Actor{TenantID: defaultTenantID, UserID: defaultUserID}
			if strings.HasPrefix(r.URL.Path, "/admin/") {
				actor.Role = identity.RolePlatformAdmin
			} else {
				actor.Role = identity.RoleTenantOwner
			}
		}

		next.ServeHTTP(w, r.WithContext(identity.WithActor(r.Context(), actor)))
	})
}

func constantTimeHeaderMatches(r *http.Request, headerName string, expected string) bool {
	provided := strings.TrimSpace(r.Header.Get(headerName))
	return provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
