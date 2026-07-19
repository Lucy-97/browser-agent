// Package jwt 网关侧 JWT 管理器，实现 internal/middleware.JWTValidator 接口。
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	pmw "github.com/Lucy-97/browser-agent/backend-gateway/internal/middleware"
)

// ErrTokenExpired 用于上层区分过期与无效。
var ErrTokenExpired = errors.New("token expired")

const (
	tokenIssuer   = "browser-agent-api"
	tokenAudience = "browser-agent-gateway"
)

// Manager 签发与校验 access/refresh token。
type Manager struct {
	secret              []byte
	accessTokenExpSec   int
	refreshTokenExpDays int
}

// claims 内部 JWT claims 结构。
type claims struct {
	MemberLevel string `json:"lvl,omitempty"`
	TenantID    string `json:"tenant_id,omitempty"`
	TenantRole  string `json:"tenant_role,omitempty"`
	jwt.RegisteredClaims
}

// NewManager 创建管理器。
func NewManager(secret string, accessExpSec, refreshExpDays int) *Manager {
	return &Manager{
		secret:              []byte(secret),
		accessTokenExpSec:   accessExpSec,
		refreshTokenExpDays: refreshExpDays,
	}
}

// IssueAccessToken 签发短期访问 token。
func (m *Manager) IssueAccessToken(userUUID, memberLevel string) (string, error) {
	return m.IssueTenantAccessToken(userUUID, memberLevel, "", "")
}

// IssueTenantAccessToken 签发带租户上下文的短期访问 token。
func (m *Manager) IssueTenantAccessToken(userUUID, memberLevel, tenantID, tenantRole string) (string, error) {
	now := time.Now()
	c := claims{
		MemberLevel: memberLevel,
		TenantID:    tenantID,
		TenantRole:  tenantRole,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userUUID,
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(m.accessTokenExpSec) * time.Second)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(m.secret)
}

// IssueRefreshToken 签发长期刷新 token。
func (m *Manager) IssueRefreshToken(userUUID string) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userUUID,
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(m.refreshTokenExpDays) * 24 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(m.secret)
}

// ValidateToken 实现 pmw.JWTValidator。过期返回 (zeroValue with Expired=true, ErrTokenExpired)。
func (m *Manager) ValidateToken(token string) (pmw.Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &claims{}, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(tokenIssuer), jwt.WithAudience(tokenAudience))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return pmw.Claims{Expired: true}, ErrTokenExpired
		}
		return pmw.Claims{}, err
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || !parsed.Valid {
		return pmw.Claims{}, errors.New("invalid token")
	}
	return pmw.Claims{
		UserUUID:    c.Subject,
		MemberLevel: c.MemberLevel,
		TenantID:    c.TenantID,
		TenantRole:  c.TenantRole,
	}, nil
}
