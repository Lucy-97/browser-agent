package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	authmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/auth"
	authrepo "github.com/Lucy-97/browser-agent/backend-api/internal/repository/auth"
)

const (
	tokenIssuer   = "browser-agent-api"
	tokenAudience = "browser-agent-gateway"
)

var (
	ErrAuthUnavailable       = errors.New("authentication is not configured")
	ErrRegistrationDisabled  = errors.New("public registration is disabled")
	ErrInvalidEmail          = errors.New("email is invalid")
	ErrInvalidPassword       = errors.New("password must contain between 12 and 72 bytes")
	ErrInvalidNickname       = errors.New("nickname is required and must not exceed 64 characters")
	ErrInvalidTenantName     = errors.New("tenant name is required and must not exceed 255 characters")
	ErrInvalidCredentials    = errors.New("email or password is incorrect")
	ErrAccountInactive       = errors.New("account is inactive")
	ErrMembershipUnavailable = errors.New("no active tenant membership is available")
	ErrInvalidAccessToken    = errors.New("access token is invalid or expired")
)

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("browser-agent-invalid-password"), bcrypt.DefaultCost)

type Repository interface {
	CreateOwner(ctx context.Context, user authmodel.User, tenant authmodel.Tenant) (authmodel.Session, error)
	UserByEmail(ctx context.Context, email string) (authmodel.User, error)
	ActiveMemberships(ctx context.Context, userID string) ([]authmodel.Membership, error)
	ActiveSession(ctx context.Context, userID string, tenantID string) (authmodel.Session, error)
	IsActiveMembership(ctx context.Context, tenantID string, userID string, role string) (bool, error)
	RecordLogin(ctx context.Context, userID string) error
}

type Options struct {
	JWTSecret               string
	AccessTokenTTL          time.Duration
	AllowPublicRegistration bool
}

type Engine struct {
	repo                    Repository
	jwtSecret               []byte
	accessTokenTTL          time.Duration
	allowPublicRegistration bool
}

type RegisterRequest struct {
	Email      string
	Password   string
	Nickname   string
	TenantName string
}

type LoginRequest struct {
	Email    string
	Password string
	TenantID string
}

type accessClaims struct {
	MemberLevel string `json:"lvl,omitempty"`
	TenantID    string `json:"tenant_id"`
	TenantRole  string `json:"tenant_role"`
	jwt.RegisteredClaims
}

func New(repo Repository, options Options) *Engine {
	ttl := options.AccessTokenTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	secret := []byte(strings.TrimSpace(options.JWTSecret))
	if len(secret) < 32 {
		secret = nil
	}
	return &Engine{
		repo:                    repo,
		jwtSecret:               secret,
		accessTokenTTL:          ttl,
		allowPublicRegistration: options.AllowPublicRegistration,
	}
}

func (engine *Engine) Register(ctx context.Context, req RegisterRequest) (authmodel.AuthResult, error) {
	if len(engine.jwtSecret) == 0 {
		return authmodel.AuthResult{}, ErrAuthUnavailable
	}
	if !engine.allowPublicRegistration {
		return authmodel.AuthResult{}, ErrRegistrationDisabled
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	if err := validatePassword(req.Password); err != nil {
		return authmodel.AuthResult{}, err
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > 64 {
		return authmodel.AuthResult{}, ErrInvalidNickname
	}
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" || utf8.RuneCountInString(tenantName) > 255 {
		return authmodel.AuthResult{}, ErrInvalidTenantName
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	session, err := engine.repo.CreateOwner(ctx, authmodel.User{
		ID:           newID("usr"),
		Email:        email,
		PasswordHash: string(passwordHash),
		Nickname:     nickname,
		MemberLevel:  "FREE",
	}, authmodel.Tenant{ID: newID("ten"), Name: tenantName})
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	return engine.issueSession(session)
}

func (engine *Engine) Login(ctx context.Context, req LoginRequest) (authmodel.AuthResult, error) {
	if len(engine.jwtSecret) == 0 {
		return authmodel.AuthResult{}, ErrAuthUnavailable
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		return authmodel.AuthResult{}, ErrInvalidCredentials
	}
	user, err := engine.repo.UserByEmail(ctx, email)
	if errors.Is(err, authrepo.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		return authmodel.AuthResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return authmodel.AuthResult{}, ErrInvalidCredentials
	}
	if user.Status != "active" {
		return authmodel.AuthResult{}, ErrAccountInactive
	}
	memberships, err := engine.repo.ActiveMemberships(ctx, user.ID)
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	membership, ok := selectMembership(memberships, strings.TrimSpace(req.TenantID))
	if !ok {
		return authmodel.AuthResult{User: user, Memberships: memberships}, ErrMembershipUnavailable
	}
	_ = engine.repo.RecordLogin(ctx, user.ID)
	return engine.issueSession(authmodel.Session{User: user, Membership: membership})
}

func (engine *Engine) SessionFromToken(ctx context.Context, token string) (authmodel.Session, error) {
	if len(engine.jwtSecret) == 0 || strings.TrimSpace(token) == "" {
		return authmodel.Session{}, ErrInvalidAccessToken
	}
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(token *jwt.Token) (any, error) {
		return engine.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(tokenIssuer), jwt.WithAudience(tokenAudience))
	if err != nil || !parsed.Valid {
		return authmodel.Session{}, ErrInvalidAccessToken
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || claims.Subject == "" || claims.TenantID == "" || claims.TenantRole == "" {
		return authmodel.Session{}, ErrInvalidAccessToken
	}
	session, err := engine.repo.ActiveSession(ctx, claims.Subject, claims.TenantID)
	if err != nil || session.Membership.Role != claims.TenantRole {
		return authmodel.Session{}, ErrInvalidAccessToken
	}
	return session, nil
}

func (engine *Engine) IsActiveMembership(ctx context.Context, tenantID string, userID string, role string) (bool, error) {
	return engine.repo.IsActiveMembership(ctx, tenantID, userID, role)
}

func (engine *Engine) AccessTokenTTL() time.Duration {
	return engine.accessTokenTTL
}

func (engine *Engine) issueSession(session authmodel.Session) (authmodel.AuthResult, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(engine.accessTokenTTL)
	claims := accessClaims{
		MemberLevel: session.User.MemberLevel,
		TenantID:    session.Membership.Tenant.ID,
		TenantRole:  session.Membership.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   session.User.ID,
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(engine.jwtSecret)
	if err != nil {
		return authmodel.AuthResult{}, err
	}
	return authmodel.AuthResult{
		AccessToken: token,
		ExpiresIn:   int(engine.accessTokenTTL.Seconds()),
		User:        session.User,
		Tenant:      session.Membership.Tenant,
		Role:        session.Membership.Role,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" || len(email) > 320 {
		return "", ErrInvalidEmail
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return ErrInvalidPassword
	}
	return nil
}

func selectMembership(memberships []authmodel.Membership, tenantID string) (authmodel.Membership, bool) {
	if tenantID == "" {
		if len(memberships) == 0 {
			return authmodel.Membership{}, false
		}
		return memberships[0], true
	}
	for _, membership := range memberships {
		if membership.Tenant.ID == tenantID {
			return membership, true
		}
	}
	return authmodel.Membership{}, false
}

func newID(prefix string) string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value[:])
}
