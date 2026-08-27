package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type (
	Admin      = model.Admin
	Session    = model.Session
	AuditEvent = model.AuditEvent
	Dashboard  = model.Dashboard
)

var ErrNotFound = model.ErrNotFound

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
			       COALESCE(array_agg(DISTINCT rp.permission_name) FILTER (WHERE rp.permission_name IS NOT NULL), '{}'),
			       EXISTS (SELECT 1 FROM admin_mfa_methods m WHERE m.admin_user_id = u.id AND m.verified_at IS NOT NULL)
		FROM admin_users u
		LEFT JOIN admin_user_roles ur ON ur.admin_user_id = u.id
		LEFT JOIN admin_role_permissions rp ON rp.role_id = ur.role_id
		`+where+`
		GROUP BY u.id`, value).Scan(&admin.ID, &admin.Email, &admin.DisplayName, &admin.PasswordHash, &admin.Status, &permissions, &admin.MFAEnrolled)
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_sessions (id, family_id, admin_user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent, mfa_verified) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, session.ID, session.FamilyID, session.AdminID, session.TokenHash, session.ExpiresAt, session.CreatedAt, session.IPAddress, session.UserAgent, session.MFAVerified)
	return err
}

func (s *PostgresStore) RotateSession(ctx context.Context, oldHash, newHash, newID string, expires time.Time) (Session, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	var old Session
	err = tx.QueryRowContext(ctx, `SELECT id, family_id, admin_user_id, revoked_at, expires_at, created_at, ip_address, user_agent, mfa_verified FROM admin_sessions WHERE refresh_token_hash = $1 FOR UPDATE`, oldHash).Scan(&old.ID, &old.FamilyID, &old.AdminID, &old.RevokedAt, &old.ExpiresAt, &old.CreatedAt, &old.IPAddress, &old.UserAgent, &old.MFAVerified)
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
	next := Session{ID: newID, FamilyID: old.FamilyID, AdminID: old.AdminID, TokenHash: newHash, ExpiresAt: expires, CreatedAt: old.CreatedAt, IPAddress: old.IPAddress, UserAgent: old.UserAgent, MFAVerified: old.MFAVerified}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_sessions (id, family_id, admin_user_id, refresh_token_hash, expires_at, created_at, ip_address, user_agent, mfa_verified) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, next.ID, next.FamilyID, next.AdminID, next.TokenHash, next.ExpiresAt, next.CreatedAt, next.IPAddress, next.UserAgent, next.MFAVerified); err != nil {
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

func (s *PostgresStore) ListSessions(ctx context.Context, adminID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, expires_at, created_at, ip_address, user_agent, mfa_verified FROM admin_sessions WHERE admin_user_id = $1 AND revoked_at IS NULL AND expires_at > NOW() ORDER BY created_at DESC, id DESC`, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var session Session
		session.AdminID = adminID
		if err := rows.Scan(&session.ID, &session.ExpiresAt, &session.CreatedAt, &session.IPAddress, &session.UserAgent, &session.MFAVerified); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *PostgresStore) RevokeSessionByID(ctx context.Context, adminID, id string, event AuditEvent) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = NOW() WHERE admin_user_id = $1 AND id = $2 AND revoked_at IS NULL`, adminID, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return false, err
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *PostgresStore) RevokeOtherSessions(ctx context.Context, adminID, currentID string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = NOW() WHERE admin_user_id = $1 AND id <> $2 AND revoked_at IS NULL`, adminID, currentID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) SessionActive(ctx context.Context, id, adminID string) (Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `SELECT id, family_id, admin_user_id, refresh_token_hash, expires_at, revoked_at, mfa_verified FROM admin_sessions WHERE id = $1 AND admin_user_id = $2 AND revoked_at IS NULL AND expires_at > NOW()`, id, adminID).Scan(&session.ID, &session.FamilyID, &session.AdminID, &session.TokenHash, &session.ExpiresAt, &session.RevokedAt, &session.MFAVerified)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return session, err
}

func (s *PostgresStore) SavePendingMFA(ctx context.Context, adminID string, secret []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO admin_mfa_methods (admin_user_id, method_type, secret_ciphertext) VALUES ($1, 'totp', $2) ON CONFLICT (admin_user_id) DO UPDATE SET secret_ciphertext = EXCLUDED.secret_ciphertext, verified_at = NULL`, adminID, secret)
	return err
}

func (s *PostgresStore) MFASecret(ctx context.Context, adminID string, verified bool) ([]byte, error) {
	query := `SELECT secret_ciphertext FROM admin_mfa_methods WHERE admin_user_id = $1`
	if verified {
		query += ` AND verified_at IS NOT NULL`
	}
	var secret []byte
	err := s.db.QueryRowContext(ctx, query, adminID).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return secret, err
}

func (s *PostgresStore) ConfirmMFA(ctx context.Context, adminID string, hashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE admin_mfa_methods SET verified_at = NOW() WHERE admin_user_id = $1 AND verified_at IS NULL`, adminID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM admin_recovery_codes WHERE admin_user_id = $1`, adminID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO admin_recovery_codes (admin_user_id, code_hash) VALUES ($1, $2)`, adminID, hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) UseRecoveryCode(ctx context.Context, adminID, code string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id, code_hash FROM admin_recovery_codes WHERE admin_user_id = $1 AND used_at IS NULL FOR UPDATE`, adminID)
	if err != nil {
		return false, err
	}
	var matched string
	for rows.Next() {
		var id, hash string
		if err = rows.Scan(&id, &hash); err != nil {
			return false, err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			matched = id
			break
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	if err = rows.Close(); err != nil {
		return false, err
	}
	if matched == "" {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE admin_recovery_codes SET used_at = NOW() WHERE id = $1 AND used_at IS NULL`, matched)
	if err != nil {
		return false, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return false, nil
	}
	return true, tx.Commit()
}

type auditExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func appendAudit(ctx context.Context, db auditExecer, event AuditEvent) error {
	_, err := db.ExecContext(ctx, `INSERT INTO admin_audit_events (admin_user_id, session_id, request_id, action, resource_type, resource_id, outcome, ip_address, user_agent, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, nullableUUID(event.AdminID), nullableUUID(event.SessionID), event.RequestID, event.Action, event.ResourceType, event.ResourceID, event.Outcome, event.IPAddress, event.UserAgent, event.OccurredAt)
	return err
}

func (s *PostgresStore) AppendAudit(ctx context.Context, event AuditEvent) error {
	return appendAudit(ctx, s.db, event)
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
