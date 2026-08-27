package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type PostgresStore struct {
	db          *sql.DB
	environment string
}

func NewPostgresStore(db *sql.DB, environment string) *PostgresStore {
	return &PostgresStore{db: db, environment: environment}
}

func (s *PostgresStore) FindAdminByEmail(ctx context.Context, email string) (Admin, error) {
	return s.findAdmin(ctx, `WHERE LOWER(u.email) = LOWER($1) AND (u.locked_until IS NULL OR u.locked_until <= NOW())`, email)
}

func (s *PostgresStore) FindAdminByID(ctx context.Context, id string) (Admin, error) {
	return s.findAdmin(ctx, `WHERE u.id = $1`, id)
}

func (s *PostgresStore) findAdmin(ctx context.Context, where string, value any) (Admin, error) {
	var admin Admin
	var permissions pq.StringArray
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.password_hash, u.status,
		       COALESCE(array_agg(DISTINCT rp.permission_name) FILTER (WHERE rp.permission_name IS NOT NULL), '{}')
		FROM admin_users u
		LEFT JOIN admin_user_roles ur ON ur.admin_user_id = u.id
		LEFT JOIN admin_role_permissions rp ON rp.role_id = ur.role_id
		`+where+`
		GROUP BY u.id`, value).Scan(&admin.ID, &admin.Email, &admin.DisplayName, &admin.PasswordHash, &admin.Status, &permissions)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	if err != nil {
		return Admin{}, fmt.Errorf("find admin: %w", err)
	}
	admin.Permissions = permissions
	return admin, nil
}

func (s *PostgresStore) RecordLoginFailure(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_users SET failed_login_count = failed_login_count + 1, locked_until = CASE WHEN failed_login_count + 1 >= 5 THEN NOW() + INTERVAL '15 minutes' ELSE locked_until END WHERE id = $1`, id)
	return err
}

func (s *PostgresStore) RecordLoginSuccess(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_users SET failed_login_count = 0, locked_until = NULL, last_login_at = NOW() WHERE id = $1 AND (locked_until IS NULL OR locked_until <= NOW())`, id)
	return err
}

func (s *PostgresStore) CreateSession(ctx context.Context, session Session) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_sessions (id, family_id, admin_user_id, refresh_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, session.ID, session.FamilyID, session.AdminID, session.TokenHash, session.ExpiresAt)
	return err
}

func (s *PostgresStore) RotateSession(ctx context.Context, oldHash, newHash, newID string, expires time.Time) (Session, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	var old Session
	err = tx.QueryRowContext(ctx, `SELECT id, family_id, admin_user_id, revoked_at, expires_at FROM admin_sessions WHERE refresh_token_hash = $1 FOR UPDATE`, oldHash).Scan(&old.ID, &old.FamilyID, &old.AdminID, &old.RevokedAt, &old.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if old.RevokedAt != nil || time.Now().After(old.ExpiresAt) {
		_, _ = tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = COALESCE(revoked_at, NOW()) WHERE family_id = $1`, old.FamilyID)
		_ = tx.Commit()
		return Session{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = NOW() WHERE id = $1`, old.ID); err != nil {
		return Session{}, err
	}
	next := Session{ID: newID, FamilyID: old.FamilyID, AdminID: old.AdminID, TokenHash: newHash, ExpiresAt: expires}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_sessions (id, family_id, admin_user_id, refresh_token_hash, expires_at) VALUES ($1, $2, $3, $4, $5)`, next.ID, next.FamilyID, next.AdminID, next.TokenHash, next.ExpiresAt); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return next, nil
}

func (s *PostgresStore) RevokeSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = COALESCE(revoked_at, NOW()) WHERE refresh_token_hash = $1`, hash)
	return err
}

func (s *PostgresStore) SessionActive(ctx context.Context, id, adminID string) (bool, error) {
	var active bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM admin_sessions WHERE id = $1 AND admin_user_id = $2 AND revoked_at IS NULL AND expires_at > NOW())`, id, adminID).Scan(&active)
	return active, err
}

func (s *PostgresStore) AppendAudit(ctx context.Context, event AuditEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_audit_events (admin_user_id, session_id, request_id, action, outcome, ip_address, user_agent, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, nullableUUID(event.AdminID), nullableUUID(event.SessionID), event.RequestID, event.Action, event.Outcome, event.IPAddress, event.UserAgent, event.OccurredAt)
	return err
}

func (s *PostgresStore) Dashboard(context.Context) (Dashboard, error) {
	return Dashboard{Status: "ready", Environment: s.environment}, nil
}

func (s *PostgresStore) Bootstrap(ctx context.Context, email, password, displayName string) error {
	if email == "" || password == "" {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO admin_users (email, password_hash, display_name) SELECT LOWER($1), $2, $3 WHERE NOT EXISTS (SELECT 1 FROM admin_users) RETURNING id`, email, string(hash), displayName).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("bootstrap refused: an administrator already exists")
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id) SELECT $1, id FROM admin_roles WHERE name = 'super_admin'`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	if _, err := uuid.Parse(value); err != nil {
		return nil
	}
	return value
}
