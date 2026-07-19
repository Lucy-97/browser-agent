package auth

import (
	"context"
	"sort"
	"sync"
	"time"

	authmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/auth"
)

type MemoryRepository struct {
	mu              sync.Mutex
	usersByID       map[string]authmodel.User
	userIDByEmail   map[string]string
	membershipsByID map[string][]authmodel.Membership
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		usersByID:       map[string]authmodel.User{},
		userIDByEmail:   map[string]string{},
		membershipsByID: map[string][]authmodel.Membership{},
	}
}

func (repo *MemoryRepository) CreateOwner(_ context.Context, user authmodel.User, tenant authmodel.Tenant) (authmodel.Session, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if _, exists := repo.userIDByEmail[user.Email]; exists {
		return authmodel.Session{}, ErrEmailAlreadyRegistered
	}
	user.Status = "active"
	user.CreatedAt = time.Now().UTC()
	tenant.Status = "active"
	membership := authmodel.Membership{Tenant: tenant, Role: "tenant_owner", Status: "active"}
	repo.usersByID[user.ID] = user
	repo.userIDByEmail[user.Email] = user.ID
	repo.membershipsByID[user.ID] = []authmodel.Membership{membership}
	return authmodel.Session{User: user, Membership: membership}, nil
}

func (repo *MemoryRepository) UserByEmail(_ context.Context, email string) (authmodel.User, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	userID, ok := repo.userIDByEmail[email]
	if !ok {
		return authmodel.User{}, ErrUserNotFound
	}
	return repo.usersByID[userID], nil
}

func (repo *MemoryRepository) ActiveMemberships(_ context.Context, userID string) ([]authmodel.Membership, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	memberships := make([]authmodel.Membership, 0)
	for _, membership := range repo.membershipsByID[userID] {
		if membership.Status == "active" && membership.Tenant.Status == "active" {
			memberships = append(memberships, membership)
		}
	}
	sort.Slice(memberships, func(i, j int) bool { return memberships[i].Tenant.ID < memberships[j].Tenant.ID })
	return memberships, nil
}

func (repo *MemoryRepository) ActiveSession(_ context.Context, userID string, tenantID string) (authmodel.Session, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	user, ok := repo.usersByID[userID]
	if !ok || user.Status != "active" {
		return authmodel.Session{}, ErrUserNotFound
	}
	for _, membership := range repo.membershipsByID[userID] {
		if membership.Tenant.ID == tenantID && membership.Status == "active" && membership.Tenant.Status == "active" {
			return authmodel.Session{User: user, Membership: membership}, nil
		}
	}
	return authmodel.Session{}, ErrMembershipNotFound
}

func (repo *MemoryRepository) IsActiveMembership(_ context.Context, tenantID string, userID string, role string) (bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	user, ok := repo.usersByID[userID]
	if !ok || user.Status != "active" {
		return false, nil
	}
	for _, membership := range repo.membershipsByID[userID] {
		if membership.Tenant.ID == tenantID && membership.Role == role && membership.Status == "active" && membership.Tenant.Status == "active" {
			return true, nil
		}
	}
	return false, nil
}

func (repo *MemoryRepository) RecordLogin(context.Context, string) error {
	return nil
}
