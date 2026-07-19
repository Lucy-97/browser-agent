package auth

import "errors"

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrUserNotFound           = errors.New("user not found")
	ErrMembershipNotFound     = errors.New("active tenant membership not found")
)
