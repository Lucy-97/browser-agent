package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	authengine "github.com/Lucy-97/browser-agent/backend-api/internal/engine/auth"
	basehandler "github.com/Lucy-97/browser-agent/backend-api/internal/handler"
	authrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/auth"
)

type Options struct {
	CookieName   string
	CookieSecure bool
}

type Handler struct {
	engine       *authengine.Engine
	cookieName   string
	cookieSecure bool
}

func New(engine *authengine.Engine, options Options) *Handler {
	cookieName := strings.TrimSpace(options.CookieName)
	if cookieName == "" {
		cookieName = "browser_agent_access"
	}
	return &Handler{engine: engine, cookieName: cookieName, cookieSecure: options.CookieSecure}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", handler.register)
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("GET /api/v1/auth/me", handler.me)
	mux.HandleFunc("POST /api/v1/auth/logout", handler.logout)
}

func (handler *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		Nickname   string `json:"nickname"`
		TenantName string `json:"tenant_name"`
	}
	if err := decodeAuthJSON(w, r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "request body is invalid", false)
		return
	}
	result, err := handler.engine.Register(r.Context(), authengine.RegisterRequest{
		Email: req.Email, Password: req.Password, Nickname: req.Nickname, TenantName: req.TenantName,
	})
	if err != nil {
		handler.writeAuthError(w, err)
		return
	}
	handler.setAccessCookie(w, result.AccessToken, result.ExpiresIn)
	basehandler.WriteJSON(w, http.StatusCreated, result)
}

func (handler *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TenantID string `json:"tenant_id"`
	}
	if err := decodeAuthJSON(w, r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "request body is invalid", false)
		return
	}
	result, err := handler.engine.Login(r.Context(), authengine.LoginRequest{
		Email: req.Email, Password: req.Password, TenantID: req.TenantID,
	})
	if err != nil {
		handler.writeAuthError(w, err)
		return
	}
	handler.setAccessCookie(w, result.AccessToken, result.ExpiresIn)
	basehandler.WriteJSON(w, http.StatusOK, result)
}

func (handler *Handler) me(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		if cookie, err := r.Cookie(handler.cookieName); err == nil {
			token = cookie.Value
		}
	}
	session, err := handler.engine.SessionFromToken(r.Context(), token)
	if err != nil {
		basehandler.WriteError(w, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "login is required", false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, session)
}

func (handler *Handler) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: handler.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: handler.cookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) setAccessCookie(w http.ResponseWriter, token string, expiresIn int) {
	http.SetCookie(w, &http.Cookie{
		Name: handler.cookieName, Value: token, Path: "/", HttpOnly: true, Secure: handler.cookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: expiresIn,
	})
}

func (handler *Handler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authengine.ErrRegistrationDisabled):
		basehandler.WriteError(w, http.StatusForbidden, "REGISTRATION_DISABLED", err.Error(), false)
	case errors.Is(err, authrepo.ErrEmailAlreadyRegistered):
		basehandler.WriteError(w, http.StatusConflict, "EMAIL_ALREADY_REGISTERED", err.Error(), false)
	case errors.Is(err, authengine.ErrInvalidEmail),
		errors.Is(err, authengine.ErrInvalidPassword),
		errors.Is(err, authengine.ErrInvalidNickname),
		errors.Is(err, authengine.ErrInvalidTenantName):
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_REGISTRATION", err.Error(), false)
	case errors.Is(err, authengine.ErrInvalidCredentials):
		basehandler.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error(), false)
	case errors.Is(err, authengine.ErrAccountInactive), errors.Is(err, authengine.ErrMembershipUnavailable):
		basehandler.WriteError(w, http.StatusForbidden, "ACCOUNT_UNAVAILABLE", err.Error(), false)
	case errors.Is(err, authengine.ErrAuthUnavailable):
		basehandler.WriteError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED", err.Error(), false)
	default:
		basehandler.WriteError(w, http.StatusInternalServerError, "AUTH_INTERNAL_ERROR", "authentication request failed", false)
	}
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}
