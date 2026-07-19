package jwt

import "testing"

func TestIssueTenantAccessTokenRoundTrip(t *testing.T) {
	manager := NewManager("test-secret", 300, 7)
	token, err := manager.IssueTenantAccessToken("user-1", "pro", "tenant-1", "tenant_owner")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserUUID != "user-1" || claims.MemberLevel != "pro" {
		t.Fatalf("user claims = %#v", claims)
	}
	if claims.TenantID != "tenant-1" || claims.TenantRole != "tenant_owner" {
		t.Fatalf("tenant claims = %#v", claims)
	}
}
