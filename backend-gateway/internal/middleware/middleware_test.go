package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubJWTValidator struct {
	claims Claims
	err    error
}

func (validator stubJWTValidator) ValidateToken(string) (Claims, error) {
	return validator.claims, validator.err
}

func TestJWTReplacesUntrustedIdentityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWT(stubJWTValidator{claims: Claims{
		UserUUID:   "trusted-user",
		TenantID:   "trusted-tenant",
		TenantRole: "tenant_owner",
	}}, JWTOptions{}))
	router.GET("/web/resource", func(c *gin.Context) {
		if got := c.GetHeader("X-User-UUID"); got != "trusted-user" {
			t.Errorf("X-User-UUID = %q", got)
		}
		if got := c.GetHeader("X-Tenant-ID"); got != "trusted-tenant" {
			t.Errorf("X-Tenant-ID = %q", got)
		}
		if got := c.GetHeader("X-Tenant-Role"); got != "tenant_owner" {
			t.Errorf("X-Tenant-Role = %q", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/web/resource", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-User-UUID", "attacker-user")
	req.Header.Set("X-Tenant-ID", "attacker-tenant")
	req.Header.Set("X-Tenant-Role", "platform_admin")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestJWTClearsIdentityHeadersOnPublicWorkerPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWT(stubJWTValidator{err: errors.New("device token is not a user JWT")}, JWTOptions{
		PublicPathPrefixes: []string{"/worker/"},
	}))
	router.GET("/worker/automation/jobs/next", func(c *gin.Context) {
		for _, header := range []string{"X-User-UUID", "X-Member-Level", "X-Tenant-ID", "X-Tenant-Role"} {
			if got := c.GetHeader(header); got != "" {
				t.Errorf("%s = %q, want empty", header, got)
			}
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/worker/automation/jobs/next", nil)
	req.Header.Set("Authorization", "Bearer worker-device-token")
	req.Header.Set("X-Tenant-ID", "attacker-tenant")
	req.Header.Set("X-Tenant-Role", "platform_admin")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
}
