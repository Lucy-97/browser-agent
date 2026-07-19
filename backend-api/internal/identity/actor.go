package identity

import (
	"context"
	"net/http"
	"strings"
)

const (
	TenantIDHeader   = "X-Tenant-ID"
	UserIDHeader     = "X-User-UUID"
	TenantRoleHeader = "X-Tenant-Role"

	RoleTenantOwner   = "tenant_owner"
	RoleTenantMember  = "tenant_member"
	RoleTenantViewer  = "tenant_viewer"
	RolePlatformAdmin = "platform_admin"
)

type Actor struct {
	TenantID string
	UserID   string
	Role     string
}

func (actor Actor) Valid() bool {
	return strings.TrimSpace(actor.TenantID) != "" && strings.TrimSpace(actor.UserID) != ""
}

func (actor Actor) HasKnownRole() bool {
	switch actor.Role {
	case RoleTenantOwner, RoleTenantMember, RoleTenantViewer, RolePlatformAdmin:
		return true
	default:
		return false
	}
}

func (actor Actor) CanWriteTenantResources() bool {
	return actor.Role == RoleTenantOwner || actor.Role == RoleTenantMember || actor.Role == RolePlatformAdmin
}

func (actor Actor) CanManageDevices() bool {
	return actor.Role == RoleTenantOwner || actor.Role == RolePlatformAdmin
}

type actorContextKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func FromRequest(r *http.Request) (Actor, bool) {
	actor, ok := r.Context().Value(actorContextKey{}).(Actor)
	return actor, ok && actor.Valid()
}

func FromHeaders(r *http.Request) Actor {
	return Actor{
		TenantID: strings.TrimSpace(r.Header.Get(TenantIDHeader)),
		UserID:   strings.TrimSpace(r.Header.Get(UserIDHeader)),
		Role:     strings.TrimSpace(r.Header.Get(TenantRoleHeader)),
	}
}
