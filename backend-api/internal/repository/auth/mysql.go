package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	authmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/auth"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repo *MySQLRepository) CreateOwner(ctx context.Context, user authmodel.User, tenant authmodel.Tenant) (authmodel.Session, error) {
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return authmodel.Session{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO users (
		uuid, email, password_hash, nickname, member_level, status, password_changed_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		user.ID, user.Email, user.PasswordHash, user.Nickname, user.MemberLevel, now, now, now,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return authmodel.Session{}, ErrEmailAlreadyRegistered
		}
		return authmodel.Session{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant (id, name, status, created_at, updated_at)
		VALUES (?, ?, 'active', ?, ?)`, tenant.ID, tenant.Name, now, now)
	if err != nil {
		return authmodel.Session{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant_membership (
		tenant_id, user_id, role, status, created_at, updated_at
	) VALUES (?, ?, 'tenant_owner', 'active', ?, ?)`, tenant.ID, user.ID, now, now)
	if err != nil {
		return authmodel.Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return authmodel.Session{}, err
	}

	user.Status = "active"
	user.CreatedAt = now
	tenant.Status = "active"
	membership := authmodel.Membership{Tenant: tenant, Role: "tenant_owner", Status: "active"}
	return authmodel.Session{User: user, Membership: membership}, nil
}

func (repo *MySQLRepository) UserByEmail(ctx context.Context, email string) (authmodel.User, error) {
	var user authmodel.User
	var platformRole sql.NullString
	err := repo.db.QueryRowContext(ctx, `SELECT uuid, email, password_hash, nickname, member_level,
		platform_role, status, created_at FROM users WHERE email = ?`, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Nickname,
		&user.MemberLevel,
		&platformRole,
		&user.Status,
		&user.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return authmodel.User{}, ErrUserNotFound
	}
	user.PlatformRole = platformRole.String
	return user, err
}

func (repo *MySQLRepository) ActiveMemberships(ctx context.Context, userID string) ([]authmodel.Membership, error) {
	rows, err := repo.db.QueryContext(ctx, `SELECT t.id, t.name, t.status, m.role, m.status
		FROM tenant_membership m
		JOIN tenant t ON t.id = m.tenant_id
		WHERE m.user_id = ? AND m.status = 'active' AND t.status = 'active'
		ORDER BY t.created_at ASC, t.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	memberships := make([]authmodel.Membership, 0)
	for rows.Next() {
		var membership authmodel.Membership
		if err := rows.Scan(
			&membership.Tenant.ID,
			&membership.Tenant.Name,
			&membership.Tenant.Status,
			&membership.Role,
			&membership.Status,
		); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	return memberships, rows.Err()
}

func (repo *MySQLRepository) ActiveSession(ctx context.Context, userID string, tenantID string) (authmodel.Session, error) {
	var session authmodel.Session
	var platformRole sql.NullString
	err := repo.db.QueryRowContext(ctx, `SELECT u.uuid, u.email, u.nickname, u.member_level, u.platform_role,
		u.status, u.created_at, t.id, t.name, t.status, m.role, m.status
		FROM users u
		JOIN tenant_membership m ON m.user_id = u.uuid
		JOIN tenant t ON t.id = m.tenant_id
		WHERE u.uuid = ? AND t.id = ?
		  AND u.status = 'active' AND m.status = 'active' AND t.status = 'active'`,
		userID, tenantID,
	).Scan(
		&session.User.ID,
		&session.User.Email,
		&session.User.Nickname,
		&session.User.MemberLevel,
		&platformRole,
		&session.User.Status,
		&session.User.CreatedAt,
		&session.Membership.Tenant.ID,
		&session.Membership.Tenant.Name,
		&session.Membership.Tenant.Status,
		&session.Membership.Role,
		&session.Membership.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return authmodel.Session{}, ErrMembershipNotFound
	}
	session.User.PlatformRole = platformRole.String
	return session, err
}

func (repo *MySQLRepository) IsActiveMembership(ctx context.Context, tenantID string, userID string, role string) (bool, error) {
	var count int
	err := repo.db.QueryRowContext(ctx, `SELECT COUNT(*)
		FROM tenant_membership m
		JOIN tenant t ON t.id = m.tenant_id
		JOIN users u ON u.uuid = m.user_id
		WHERE m.tenant_id = ? AND m.user_id = ? AND m.role = ?
		  AND m.status = 'active' AND t.status = 'active' AND u.status = 'active'`,
		tenantID, userID, role,
	).Scan(&count)
	return count == 1, err
}

func (repo *MySQLRepository) RecordLogin(ctx context.Context, userID string) error {
	_, err := repo.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE uuid = ?`, time.Now().UTC(), userID)
	return err
}
