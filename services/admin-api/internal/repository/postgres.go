package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type (
	Admin           = model.Admin
	Role            = model.Role
	Permission      = model.Permission
	Invitation      = model.Invitation
	Session         = model.Session
	AuditEvent      = model.AuditEvent
	AuditFilter     = model.AuditFilter
	AuditEventPage  = model.AuditEventPage
	Dashboard       = model.Dashboard
	User            = model.User
	UserPage        = model.UserPage
	UserDetail      = model.UserDetail
	Suspension      = model.Suspension
	Room            = model.Room
	RoomPlayer      = model.RoomPlayer
	RoomFilter      = model.RoomFilter
	RoomPage        = model.RoomPage
	RoomDetail      = model.RoomDetail
	Game            = model.Game
	GamePlayer      = model.GamePlayer
	GameMove        = model.GameMove
	GameCard        = model.GameCard
	GameFlag        = model.GameFlag
	GameNote        = model.GameNote
	GameDetail      = model.GameDetail
	GameFilter      = model.GameFilter
	GamePage        = model.GamePage
	TimeWindow      = model.TimeWindow
	ActivitySummary = model.ActivitySummary
	OperationsLink  = model.OperationsLink
	ServiceHealth   = model.ServiceHealth
)

var (
	ErrNotFound   = model.ErrNotFound
	ErrConflict   = model.ErrConflict
	ErrUserActive = model.ErrUserActive
)

type PostgresStore struct {
	db              *sql.DB
	environment     string
	apiHealthURL    string
	wsHealthURL     string
	operationsLinks map[string]string
	httpClient      *http.Client
}

func NewPostgresStore(db *sql.DB, environment string, options ...DashboardOptions) *PostgresStore {
	store := &PostgresStore{db: db, environment: environment, httpClient: &http.Client{Timeout: 2 * time.Second}}
	if len(options) > 0 {
		store.apiHealthURL = options[0].APIHealthURL
		store.wsHealthURL = options[0].WSHealthURL
		store.operationsLinks = options[0].OperationsLinks
	}
	return store
}

type DashboardOptions struct {
	APIHealthURL    string
	WSHealthURL     string
	OperationsLinks map[string]string
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
	if len(event.Metadata) > 16*1024 || len(event.BeforeState) > 64*1024 || len(event.AfterState) > 64*1024 {
		return errors.New("audit payload exceeds size limit")
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	_, err := db.ExecContext(ctx, `INSERT INTO admin_audit_events (id, admin_user_id, session_id, request_id, action, resource_type, resource_id, reason, outcome, before_state, after_state, metadata, ip_address, user_agent, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`, event.ID, nullableUUID(event.AdminID), nullableUUID(event.SessionID), event.RequestID, event.Action, event.ResourceType, event.ResourceID, event.Reason, event.Outcome, nullableJSON(event.BeforeState), nullableJSON(event.AfterState), nullableJSON(event.Metadata), event.IPAddress, event.UserAgent, event.OccurredAt)
	return err
}

func (s *PostgresStore) AppendAudit(ctx context.Context, event AuditEvent) error {
	return appendAudit(ctx, s.db, event)
}

func (s *PostgresStore) ListAuditEvents(ctx context.Context, filter AuditFilter) (AuditEventPage, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	clauses := []string{"TRUE"}
	args := []any{}
	add := func(column string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filter.ID != "" {
		add("id", filter.ID)
	}
	if filter.ActorID != "" {
		add("admin_user_id", filter.ActorID)
	}
	if filter.Action != "" {
		add("action", filter.Action)
	}
	if filter.ResourceType != "" {
		add("resource_type", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		add("resource_id", filter.ResourceID)
	}
	if filter.Outcome != "" {
		add("outcome", filter.Outcome)
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		clauses = append(clauses, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		clauses = append(clauses, fmt.Sprintf("occurred_at < $%d", len(args)))
	}
	args = append(args, limit, filter.Offset)
	query := `SELECT id, admin_user_id, session_id, request_id, action, resource_type, resource_id, reason, outcome, before_state, after_state, metadata, ip_address, user_agent, occurred_at FROM admin_audit_events WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(" ORDER BY occurred_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return AuditEventPage{}, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		var adminID, sessionID, requestID, resourceType, resourceID, reason, ipAddress, userAgent sql.NullString
		var beforeState, afterState, metadata []byte
		if err := rows.Scan(&event.ID, &adminID, &sessionID, &requestID, &event.Action, &resourceType, &resourceID, &reason, &event.Outcome, &beforeState, &afterState, &metadata, &ipAddress, &userAgent, &event.OccurredAt); err != nil {
			return AuditEventPage{}, err
		}
		event.AdminID, event.SessionID, event.RequestID, event.ResourceType, event.ResourceID, event.Reason, event.IPAddress, event.UserAgent = adminID.String, sessionID.String, requestID.String, resourceType.String, resourceID.String, reason.String, ipAddress.String, userAgent.String
		event.BeforeState, event.AfterState, event.Metadata = beforeState, afterState, metadata
		events = append(events, event)
	}
	return AuditEventPage{Events: events, Limit: limit, Offset: filter.Offset}, rows.Err()
}

func (s *PostgresStore) SearchUsers(ctx context.Context, query string, limit, offset int, sensitive bool) (UserPage, error) {
	args := []any{limit, offset}
	where := "TRUE"
	if query != "" {
		args = append([]any{"%" + strings.ToLower(query) + "%"}, args...)
		where = "(CAST(u.id AS TEXT) ILIKE $1 OR u.username ILIKE $1 OR u.display_name ILIKE $1"
		if sensitive {
			where += " OR LOWER(COALESCE(u.email, '')) LIKE $1"
		}
		where += ")"
	}
	email := "NULL::TEXT"
	if sensitive {
		email = "u.email"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.id, u.username, u.display_name, u.version, `+email+`, u.created_at, CASE WHEN u.suspended_at IS NOT NULL AND (u.suspension_expires_at IS NULL OR u.suspension_expires_at > NOW()) THEN u.suspension_reason END, CASE WHEN u.suspended_at IS NOT NULL AND (u.suspension_expires_at IS NULL OR u.suspension_expires_at > NOW()) THEN u.suspension_expires_at END FROM users u WHERE `+where+` ORDER BY u.created_at DESC, u.id DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return UserPage{}, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()
	users := make([]User, 0, limit)
	for rows.Next() {
		var user User
		var email sql.NullString
		var reason sql.NullString
		var expiresAt sql.NullTime
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Version, &email, &user.CreatedAt, &reason, &expiresAt); err != nil {
			return UserPage{}, fmt.Errorf("scan user: %w", err)
		}
		user.Email = email.String
		if reason.Valid {
			user.Suspension = &Suspension{Reason: reason.String}
			if expiresAt.Valid {
				user.Suspension.ExpiresAt = &expiresAt.Time
			}
		}
		users = append(users, user)
	}
	return UserPage{Users: users, Limit: limit, Offset: offset}, rows.Err()
}

func (s *PostgresStore) GetUser(ctx context.Context, id string, sensitive bool) (UserDetail, error) {
	email := "NULL::TEXT"
	if sensitive {
		email = "u.email"
	}
	var result UserDetail
	var nullableEmail sql.NullString
	var reason sql.NullString
	var expiresAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.version, `+email+`, u.created_at, CASE WHEN u.suspended_at IS NOT NULL AND (u.suspension_expires_at IS NULL OR u.suspension_expires_at > NOW()) THEN u.suspension_reason END, CASE WHEN u.suspended_at IS NOT NULL AND (u.suspension_expires_at IS NULL OR u.suspension_expires_at > NOW()) THEN u.suspension_expires_at END FROM users u WHERE u.id = $1`, id).Scan(&result.User.ID, &result.User.Username, &result.User.DisplayName, &result.User.Version, &nullableEmail, &result.User.CreatedAt, &reason, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UserDetail{}, ErrNotFound
	}
	if err != nil {
		return UserDetail{}, fmt.Errorf("get user: %w", err)
	}
	result.User.Email = nullableEmail.String
	if reason.Valid {
		result.User.Suspension = &Suspension{Reason: reason.String}
		if expiresAt.Valid {
			result.User.Suspension.ExpiresAt = &expiresAt.Time
		}
	}
	result.Providers, err = stringList(ctx, s.db, `SELECT provider FROM user_providers WHERE user_id = $1 ORDER BY provider ASC`, id)
	if err != nil {
		return UserDetail{}, err
	}
	result.Stats = map[string]any{}
	var gamesPlayed, wins int64
	var totalPenalty, xp int64
	if err = s.db.QueryRowContext(ctx, `SELECT games_played, wins, total_penalty, xp FROM user_stats WHERE user_id = $1`, id).Scan(&gamesPlayed, &wins, &totalPenalty, &xp); err == nil {
		result.Stats = map[string]any{"games_played": gamesPlayed, "wins": wins, "total_penalty": totalPenalty, "xp": xp}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return UserDetail{}, err
	}
	result.Ratings, err = objectList(ctx, s.db, `SELECT rating_before, rating_after, rating_delta, created_at FROM player_rating_events WHERE user_id = $1 ORDER BY created_at DESC, game_id DESC LIMIT 100`, id, []string{"rating_before", "rating_after", "rating_delta", "created_at"})
	if err != nil {
		return UserDetail{}, err
	}
	result.Achievements, err = objectList(ctx, s.db, `SELECT achievement_id, earned_at FROM user_achievements WHERE user_id = $1 ORDER BY earned_at DESC, achievement_id DESC`, id, []string{"achievement_id", "earned_at"})
	if err != nil {
		return UserDetail{}, err
	}
	result.Skins, err = objectList(ctx, s.db, `SELECT s.id, s.name, us.earned_at FROM user_skins us JOIN skins s ON s.id = us.skin_id WHERE us.user_id = $1 ORDER BY us.earned_at DESC, s.id DESC`, id, []string{"id", "name", "earned_at"})
	if err != nil {
		return UserDetail{}, err
	}
	result.Games, err = objectList(ctx, s.db, `SELECT g.id, g.room_id, g.finished_at, gp.penalty_points, gp.rank FROM game_players gp JOIN games g ON g.id = gp.game_id WHERE gp.user_id = $1 ORDER BY g.finished_at DESC, g.id DESC LIMIT 100`, id, []string{"id", "room_id", "finished_at", "penalty_points", "rank"})
	if err != nil {
		return UserDetail{}, err
	}
	result.Room, err = objectOne(ctx, s.db, `SELECT r.id, r.status, r.created_at FROM room_players rp JOIN rooms r ON r.id = rp.room_id WHERE rp.user_id = $1 AND r.status IN ('waiting', 'in_progress') ORDER BY r.created_at DESC LIMIT 1`, id, []string{"id", "status", "created_at"})
	if err != nil {
		return UserDetail{}, err
	}
	return result, nil
}

func (s *PostgresStore) SuspendUser(ctx context.Context, id string, suspension Suspension, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET suspended_at = NOW(), suspension_expires_at = $2, suspension_reason = $3 WHERE id = $1`, id, suspension.ExpiresAt, suspension.Reason)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, id); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) UpdateUserDisplayName(ctx context.Context, id, displayName string, version int, event AuditEvent) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var user User
	var oldDisplayName string
	err = tx.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id = $1`, id).Scan(&oldDisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM room_players rp JOIN rooms r ON r.id = rp.room_id WHERE rp.user_id = $1 AND r.status IN ('waiting', 'in_progress'))`, id).Scan(&active); err != nil {
		return User{}, err
	}
	if active {
		return User{}, ErrUserActive
	}
	err = tx.QueryRowContext(ctx, `UPDATE users SET display_name = $2, version = version + 1 WHERE id = $1 AND version = $3 RETURNING id, username, display_name, version, created_at`, id, displayName, version).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Version, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, err
	}
	event.BeforeState = []byte(fmt.Sprintf(`{"display_name":%q,"version":%d}`, oldDisplayName, version))
	event.AfterState = []byte(fmt.Sprintf(`{"display_name":%q,"version":%d}`, displayName, user.Version))
	if err := appendAudit(ctx, tx, event); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *PostgresStore) ReinstateUser(ctx context.Context, id string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET suspended_at = NULL, suspension_expires_at = NULL, suspension_reason = NULL WHERE id = $1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func stringList(ctx context.Context, db *sql.DB, query, id string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func objectList(ctx context.Context, db *sql.DB, query, id string, fields []string) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(fields))
		pointers := make([]any, len(fields))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, field := range fields {
			item[field] = values[i]
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func objectOne(ctx context.Context, db *sql.DB, query, id string, fields []string) (map[string]any, error) {
	items, err := objectList(ctx, db, query, id, fields)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func (s *PostgresStore) Dashboard(ctx context.Context) (Dashboard, error) {
	now := time.Now().UTC()
	day := TimeWindow{From: now.Truncate(24 * time.Hour), To: now.Truncate(24 * time.Hour).Add(24 * time.Hour)}
	month := TimeWindow{From: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), To: time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)}

	current, err := s.currentActivity(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	daily, err := s.activitySummary(ctx, day)
	if err != nil {
		return Dashboard{}, err
	}
	monthly, err := s.activitySummary(ctx, month)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{
		Status: "ready", Environment: s.environment,
		Windows: model.DashboardWindows{Day: day, Month: month}, Current: current,
		Daily: daily, Monthly: monthly,
		Services: model.DashboardServices{API: s.health(ctx, s.apiHealthURL), WS: s.health(ctx, s.wsHealthURL)},
		Links:    dashboardLinks(s.operationsLinks),
	}, nil
}

func (s *PostgresStore) currentActivity(ctx context.Context) (model.CurrentActivity, error) {
	var activity model.CurrentActivity
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(DISTINCT rp.user_id) FROM room_players rp JOIN rooms r ON r.id = rp.room_id WHERE r.status IN ('waiting', 'in_progress')),
			(SELECT COUNT(*) FROM rooms WHERE status IN ('waiting', 'in_progress')),
			(SELECT COUNT(*) FROM rooms WHERE status = 'in_progress')
	`).Scan(&activity.Players, &activity.Rooms, &activity.Games)
	return activity, err
}

func (s *PostgresStore) activitySummary(ctx context.Context, window TimeWindow) (ActivitySummary, error) {
	var summary ActivitySummary
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE created_at >= $1 AND created_at < $2),
			(SELECT COUNT(DISTINCT rp.user_id) FROM room_players rp JOIN rooms r ON r.id = rp.room_id WHERE rp.joined_at >= $1 AND rp.joined_at < $2),
			(SELECT COUNT(*) FROM rooms WHERE created_at >= $1 AND created_at < $2),
			(SELECT COUNT(*) FROM games WHERE started_at >= $1 AND started_at < $2),
			(SELECT COUNT(*) FROM games WHERE finished_at >= $1 AND finished_at < $2),
			0::BIGINT,
			COALESCE((SELECT AVG(EXTRACT(EPOCH FROM finished_at - started_at))::BIGINT FROM games WHERE finished_at >= $1 AND finished_at < $2), 0)
	`, window.From, window.To).Scan(
		&summary.Registrations, &summary.Players, &summary.Rooms, &summary.GamesStarted,
		&summary.GamesCompleted, &summary.GamesAbandoned, &summary.AverageGameDurationSeconds,
	)
	return summary, err
}

func (s *PostgresStore) health(ctx context.Context, rawURL string) ServiceHealth {
	if rawURL == "" {
		return ServiceHealth{Status: "not_configured"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ServiceHealth{Status: "unreachable"}
	}
	response, err := s.httpClient.Do(req)
	if err != nil {
		return ServiceHealth{Status: "unreachable"}
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return ServiceHealth{Status: "ok"}
	}
	return ServiceHealth{Status: "degraded"}
}

func dashboardLinks(links map[string]string) []OperationsLink {
	result := make([]OperationsLink, 0, len(links))
	for name, rawURL := range links {
		if rawURL != "" {
			result = append(result, OperationsLink{Name: name, URL: rawURL})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
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

func (s *PostgresStore) RevokeAdminSessions(ctx context.Context, adminID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = COALESCE(revoked_at, NOW()) WHERE admin_user_id = $1 AND revoked_at IS NULL`, adminID)
	return err
}

func (s *PostgresStore) ListAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.created_at,
		       COALESCE(array_agg(DISTINCT rp.permission_name) FILTER (WHERE rp.permission_name IS NOT NULL), '{}'),
		       EXISTS (SELECT 1 FROM admin_mfa_methods m WHERE m.admin_user_id = u.id AND m.verified_at IS NOT NULL)
		FROM admin_users u
		LEFT JOIN admin_user_roles ur ON ur.admin_user_id = u.id
		LEFT JOIN admin_role_permissions rp ON rp.role_id = ur.role_id
		GROUP BY u.id
		ORDER BY u.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	defer rows.Close()
	var admins []Admin
	for rows.Next() {
		var a Admin
		var perms pq.StringArray
		if err := rows.Scan(&a.ID, &a.Email, &a.DisplayName, &a.Status, &a.CreatedAt, &perms, &a.MFAEnrolled); err != nil {
			return nil, fmt.Errorf("scan admin: %w", err)
		}
		a.Permissions = perms
		admins = append(admins, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list admins rows: %w", err)
	}

	for i := range admins {
		roleRows, err := s.db.QueryContext(ctx, `
			SELECT r.id, r.name, r.description
			FROM admin_roles r
			JOIN admin_user_roles ur ON ur.role_id = r.id
			WHERE ur.admin_user_id = $1
			ORDER BY r.name ASC
		`, admins[i].ID)
		if err != nil {
			return nil, fmt.Errorf("list admin roles: %w", err)
		}
		for roleRows.Next() {
			var r Role
			if err := roleRows.Scan(&r.ID, &r.Name, &r.Description); err != nil {
				roleRows.Close()
				return nil, fmt.Errorf("scan role: %w", err)
			}
			admins[i].Roles = append(admins[i].Roles, r)
		}
		roleRows.Close()
	}
	return admins, nil
}

func (s *PostgresStore) CreateInvitation(ctx context.Context, inv Invitation, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE LOWER(email) = LOWER($1))`, inv.Email).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check existing admin email: %w", err)
	}
	if exists {
		return ErrConflict
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM admin_invitations WHERE LOWER(email) = LOWER($1) AND accepted_at IS NULL AND expires_at > NOW())`, inv.Email).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check existing active invitation: %w", err)
	}
	if exists {
		return ErrConflict
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO admin_invitations (id, email, token_hash, role_id, invited_by_admin_id, expires_at, created_at)
		VALUES ($1, LOWER($2), $3, $4, $5, $6, $7)
	`, inv.ID, inv.Email, inv.TokenHash, inv.RoleID, nullableUUID(inv.InvitedBy), inv.ExpiresAt, inv.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert invitation: %w", err)
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return tx.Commit()
}

func (s *PostgresStore) FindInvitationByTokenHash(ctx context.Context, tokenHash string) (Invitation, error) {
	var inv Invitation
	var invitedBy sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT i.id, i.email, i.role_id, r.name, i.invited_by_admin_id, i.expires_at, i.accepted_at, i.created_at
		FROM admin_invitations i
		JOIN admin_roles r ON r.id = i.role_id
		WHERE i.token_hash = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
	`, tokenHash).Scan(&inv.ID, &inv.Email, &inv.RoleID, &inv.RoleName, &invitedBy, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Invitation{}, ErrNotFound
	}
	if err != nil {
		return Invitation{}, fmt.Errorf("find invitation: %w", err)
	}
	if invitedBy.Valid {
		inv.InvitedBy = invitedBy.String
	}
	return inv, nil
}

func (s *PostgresStore) AcceptInvitation(ctx context.Context, tokenHash, displayName, passwordHash string, event AuditEvent) (Admin, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Admin{}, err
	}
	defer tx.Rollback()

	var inv Invitation
	var invitedBy sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT i.id, i.email, i.role_id, r.name, i.invited_by_admin_id, i.expires_at, i.accepted_at, i.created_at
		FROM admin_invitations i
		JOIN admin_roles r ON r.id = i.role_id
		WHERE i.token_hash = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		FOR UPDATE
	`, tokenHash).Scan(&inv.ID, &inv.Email, &inv.RoleID, &inv.RoleName, &invitedBy, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	if err != nil {
		return Admin{}, fmt.Errorf("lock invitation: %w", err)
	}

	var adminID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO admin_users (email, display_name, password_hash, status)
		VALUES (LOWER($1), $2, $3, 'active')
		RETURNING id
	`, inv.Email, displayName, passwordHash).Scan(&adminID)
	if err != nil {
		return Admin{}, fmt.Errorf("create invited admin: %w", err)
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id) VALUES ($1, $2)`, adminID, inv.RoleID)
	if err != nil {
		return Admin{}, fmt.Errorf("assign role to invited admin: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE admin_invitations SET accepted_at = NOW() WHERE id = $1`, inv.ID)
	if err != nil {
		return Admin{}, fmt.Errorf("mark invitation accepted: %w", err)
	}
	event.AdminID = adminID
	event.ResourceID = adminID
	event.AfterState = []byte(`{"status":"active"}`)
	if err := appendAudit(ctx, tx, event); err != nil {
		return Admin{}, fmt.Errorf("append audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Admin{}, err
	}
	return s.FindAdminByID(ctx, adminID)
}

func (s *PostgresStore) SetAdminStatus(ctx context.Context, id, status string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE admin_users SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("set admin status: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil || count != 1 {
		return ErrNotFound
	}
	if status == "disabled" {
		if _, err := tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = COALESCE(revoked_at, NOW()) WHERE admin_user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return fmt.Errorf("revoke disabled sessions: %w", err)
		}
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return tx.Commit()
}

func (s *PostgresStore) SetAdminRoles(ctx context.Context, id string, roleIDs []string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE id = $1)`, id).Scan(&exists)
	if err != nil || !exists {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM admin_user_roles WHERE admin_user_id = $1`, id); err != nil {
		return fmt.Errorf("delete old roles: %w", err)
	}
	for _, rid := range roleIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id) VALUES ($1, $2)`, id, rid); err != nil {
			return fmt.Errorf("insert admin role: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = COALESCE(revoked_at, NOW()) WHERE admin_user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		return fmt.Errorf("revoke modified admin sessions: %w", err)
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return tx.Commit()
}

func (s *PostgresStore) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.name, r.description,
		       COALESCE(array_agg(rp.permission_name ORDER BY rp.permission_name) FILTER (WHERE rp.permission_name IS NOT NULL), '{}')
		FROM admin_roles r
		LEFT JOIN admin_role_permissions rp ON rp.role_id = r.id
		GROUP BY r.id
		ORDER BY r.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	var roles []Role
	for rows.Next() {
		var r Role
		var perms pq.StringArray
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &perms); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		r.Permissions = perms
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func (s *PostgresStore) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, description FROM admin_permissions ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()
	var perms []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (s *PostgresStore) UpdateRolePermissions(ctx context.Context, roleID string, permissions []string, event AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM admin_roles WHERE id = $1)`, roleID).Scan(&exists)
	if err != nil || !exists {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM admin_role_permissions WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("delete role permissions: %w", err)
	}
	for _, p := range permissions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO admin_role_permissions (role_id, permission_name) VALUES ($1, $2)`, roleID, p); err != nil {
			return fmt.Errorf("insert role permission: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE admin_sessions s
		SET revoked_at = COALESCE(s.revoked_at, NOW())
		FROM admin_user_roles ur
		WHERE ur.role_id = $1 AND ur.admin_user_id = s.admin_user_id AND s.revoked_at IS NULL
	`, roleID); err != nil {
		return fmt.Errorf("revoke sessions affected by role permissions: %w", err)
	}
	if err := appendAudit(ctx, tx, event); err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return tx.Commit()
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
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
