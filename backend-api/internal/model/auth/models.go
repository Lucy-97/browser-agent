package auth

import "time"

type User struct {
	ID           string    `json:"user_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Nickname     string    `json:"nickname"`
	MemberLevel  string    `json:"member_level"`
	PlatformRole string    `json:"-"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Tenant struct {
	ID     string `json:"tenant_id"`
	Name   string `json:"tenant_name"`
	Status string `json:"status"`
}

type Membership struct {
	Tenant Tenant `json:"tenant"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type Session struct {
	User       User       `json:"user"`
	Membership Membership `json:"membership"`
}

type AuthResult struct {
	AccessToken string       `json:"access_token"`
	ExpiresIn   int          `json:"expires_in"`
	User        User         `json:"user"`
	Tenant      Tenant       `json:"tenant"`
	Role        string       `json:"role"`
	Memberships []Membership `json:"memberships,omitempty"`
}
