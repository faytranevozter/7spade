package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type User struct {
	ID                   uuid.UUID
	Email                sql.NullString
	PasswordHash         sql.NullString
	DisplayName          string
	Username             string
	CreatedAt            time.Time
	EmailVerifiedAt      sql.NullTime
	DeletionScheduledAt  sql.NullTime
}

// DeletedUserDisplayName is written onto historical game_players seats before
// the user row is hard-deleted during account finalization.
const DeletedUserDisplayName = "Deleted User"

// AccountDeletionGracePeriod is how long a scheduled deletion can be cancelled
// before the background finalizer hard-deletes the account.
const AccountDeletionGracePeriod = 7 * 24 * time.Hour

type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	Username       string
	AvatarURL      string
}

// UserProvider is one linked identity provider for an account.
type UserProvider struct {
	Provider  string
	AvatarURL *string
	CreatedAt time.Time
}

// ErrUsernameTaken is returned when an insert/update violates the unique
// username constraint.
var ErrUsernameTaken = errors.New("username taken")

// ErrEmailTaken is returned when an insert violates the unique email
// constraint, including races after the registration pre-check.
var ErrEmailTaken = errors.New("email taken")

func CreateUser(db *sql.DB, email, passwordHash, displayName, username string) (*User, error) {
	user := &User{ID: uuid.New(), Email: sql.NullString{String: email, Valid: true}, PasswordHash: sql.NullString{String: passwordHash, Valid: true}, DisplayName: displayName, Username: username, CreatedAt: time.Now()}
	err := db.QueryRow(`
		INSERT INTO users (id, email, password_hash, display_name, username, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, password_hash, display_name, username, created_at
	`, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.Username, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		if isUniqueViolation(err, "idx_users_username") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`SELECT id, email, password_hash, display_name, username, created_at, email_verified_at, deletion_scheduled_at FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt, &user.EmailVerifiedAt, &user.DeletionScheduledAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func GetUserByID(db *sql.DB, id uuid.UUID) (*User, error) {
	user := &User{}
	err := db.QueryRow(`SELECT id, email, password_hash, display_name, username, created_at, email_verified_at, deletion_scheduled_at FROM users WHERE id = $1`, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt, &user.EmailVerifiedAt, &user.DeletionScheduledAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// GetUserByUsername returns the single user with the given normalized username,
// or (nil, nil) when none exists. Username is unique, so there's at most one.
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`SELECT id, email, password_hash, display_name, username, created_at FROM users WHERE username = $1`, username).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return user, nil
}

// UserSearchResult is the public-only shape returned by user search: no email,
// no stats, just enough to render a result row and send a friend request by id.
type UserSearchResult struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

// escapeLikePattern escapes the LIKE wildcards (%, _) and the escape char itself
// so user-supplied search text is matched literally rather than as a pattern.
func escapeLikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// SearchUsers finds registered users whose username or display_name matches the
// query (case-insensitive substring), excluding the caller and anyone in a
// blocked relationship with them (either direction). Results are relevance
// ranked — exact username, then prefix, then substring — and capped at limit.
// query must already be trimmed; the caller enforces a minimum length.
func SearchUsers(db *sql.DB, query string, excludeID uuid.UUID, limit int) ([]UserSearchResult, error) {
	escaped := escapeLikePattern(query)
	contains := "%" + escaped + "%"
	prefix := escaped + "%"

	rows, err := db.Query(`
		SELECT u.id, u.username, u.display_name, av.avatar_url
		FROM users u`+avatarLateralJoin+`
		WHERE u.id <> $1
		  AND (u.username ILIKE $2 ESCAPE '\' OR u.display_name ILIKE $2 ESCAPE '\')
		  AND NOT EXISTS (
		      SELECT 1 FROM friendships f
		      WHERE f.status = 'blocked'
		        AND ((f.requester_id = $1 AND f.addressee_id = u.id)
		          OR (f.requester_id = u.id AND f.addressee_id = $1))
		  )
		ORDER BY
		  CASE
		    WHEN lower(u.username) = lower($4) THEN 0
		    WHEN u.username ILIKE $3 ESCAPE '\' THEN 1
		    ELSE 2
		  END,
		  u.username ASC
		LIMIT $5
	`, excludeID, contains, prefix, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	results := []UserSearchResult{}
	for rows.Next() {
		var (
			id     uuid.UUID
			r      UserSearchResult
			avatar sql.NullString
		)
		if err := rows.Scan(&id, &r.Username, &r.DisplayName, &avatar); err != nil {
			return nil, fmt.Errorf("scan user search: %w", err)
		}
		r.UserID = id.String()
		if avatar.Valid {
			r.AvatarURL = &avatar.String
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user search: %w", err)
	}
	return results, nil
}

// UpdateDisplayName updates a user's display name and returns the updated row.
// display_name has no uniqueness constraint (only username is unique), so this
// is a plain update. Returns (nil, nil) when no user matches the id.
func UpdateDisplayName(db *sql.DB, id uuid.UUID, displayName string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		UPDATE users SET display_name = $1 WHERE id = $2
		RETURNING id, email, password_hash, display_name, username, created_at
	`, displayName, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update display name: %w", err)
	}
	return user, nil
}

// UpdatePasswordHash sets a new bcrypt hash for the user. Returns sql.ErrNoRows
// when no user matches.
func UpdatePasswordHash(db *sql.DB, id uuid.UUID, passwordHash string) error {
	res, err := db.Exec(`UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update password hash rows: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// MarkEmailVerified stamps email_verified_at = NOW() for the user (idempotent).
// Returns sql.ErrNoRows when no user matches.
func MarkEmailVerified(db *sql.DB, id uuid.UUID) error {
	res, err := db.Exec(`UPDATE users SET email_verified_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark email verified rows: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505). When constraint is non-empty it also requires the
// violation to be on that constraint/index, so callers can distinguish which
// unique key was hit. It prefers the structured *pq.Error fields and falls back
// to string matching for safety.
func isUniqueViolation(err error, constraint string) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if pqErr.Code != "23505" {
			return false
		}
		if constraint == "" {
			return true
		}
		return pqErr.Constraint == constraint
	}
	// Fallback: inspect the error string when the driver error isn't a
	// *pq.Error (e.g. a wrapped/mocked error in tests).
	msg := err.Error()
	if !strings.Contains(msg, "23505") && !strings.Contains(msg, "duplicate key value") {
		return false
	}
	if constraint == "" {
		return true
	}
	return strings.Contains(msg, constraint)
}

// avatarLateralJoin selects a single avatar per user from user_providers, using
// provider precedence (google > github > telegram) and newest link as tiebreak.
// Use it as `... <base query with alias u> ` + avatarLateralJoin and select
// `av.avatar_url`. A LATERAL ... LIMIT 1 keeps one row per user, so a
// multi-provider user never multiplies rows. Yields NULL when the user has no
// provider avatar (email/password-only users).
const avatarLateralJoin = `
	LEFT JOIN LATERAL (
		SELECT up.avatar_url
		FROM user_providers up
		WHERE up.user_id = u.id AND up.avatar_url IS NOT NULL
		ORDER BY CASE up.provider
		           WHEN 'google'   THEN 0
		           WHEN 'github'   THEN 1
		           WHEN 'telegram' THEN 2
		           ELSE 3
		         END,
		         up.created_at DESC
		LIMIT 1
	) av ON true
`

// GetUserAvatar resolves the single preferred avatar URL for a user, or nil when
// they have no provider avatar. Used to denormalize the avatar into the JWT at
// login/register/refresh (the OAuth callback already has it from the provider).
func GetUserAvatar(db *sql.DB, userID uuid.UUID) (*string, error) {
	var avatar sql.NullString
	err := db.QueryRow(`
		SELECT av.avatar_url
		FROM users u`+avatarLateralJoin+`
		WHERE u.id = $1
	`, userID).Scan(&avatar)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user avatar: %w", err)
	}
	if !avatar.Valid {
		return nil, nil
	}
	return &avatar.String, nil
}

// ListUserProviders returns linked OAuth providers for the user, newest first.
// AvatarURL is nil when the provider account has no avatar.
func ListUserProviders(db *sql.DB, userID uuid.UUID) ([]UserProvider, error) {
	rows, err := db.Query(`
		SELECT provider, avatar_url, created_at
		FROM user_providers
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user providers: %w", err)
	}
	defer rows.Close()

	providers := []UserProvider{}
	for rows.Next() {
		var (
			p      UserProvider
			avatar sql.NullString
		)
		if err := rows.Scan(&p.Provider, &avatar, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user provider: %w", err)
		}
		if avatar.Valid {
			p.AvatarURL = &avatar.String
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user providers: %w", err)
	}

	return providers, nil
}

func UpsertOAuthUser(db *sql.DB, profile OAuthProfile) (*User, error) {
	if profile.Provider == "" || profile.ProviderUserID == "" {
		return nil, fmt.Errorf("provider and provider_user_id are required")
	}
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	if profile.DisplayName == "" {
		if profile.Email != "" {
			profile.DisplayName = profile.Email
		} else {
			profile.DisplayName = profile.Provider + " user"
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var userID uuid.UUID
	err = tx.QueryRow(`SELECT user_id FROM user_providers WHERE provider = $1 AND provider_id = $2`, profile.Provider, profile.ProviderUserID).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("lookup provider: %w", err)
	}
	if err == sql.ErrNoRows && profile.Email != "" {
		err2 := tx.QueryRow(`SELECT id FROM users WHERE email = $1`, profile.Email).Scan(&userID)
		if err2 != nil && err2 != sql.ErrNoRows {
			return nil, fmt.Errorf("lookup email: %w", err2)
		}
	}

	var user User
	if userID == uuid.Nil {
		var email sql.NullString
		if profile.Email != "" {
			email = sql.NullString{String: profile.Email, Valid: true}
		}
		// New OAuth users get an auto-generated lowercase username. Try the
		// provider handle first, then the email local-part, then the display
		// name, falling back to "player" with a numeric suffix on collision.
		emailLocal := profile.Email
		if at := strings.IndexByte(emailLocal, '@'); at > 0 {
			emailLocal = emailLocal[:at]
		}
		// GenerateUniqueUsername races with concurrent signups (its EXISTS check
		// and this INSERT aren't atomic), so retry a few times on a unique
		// violation. Each attempt is wrapped in a savepoint because a failed
		// statement otherwise aborts the whole transaction.
		newID := uuid.New()
		const maxAttempts = 5
		for attempt := 0; attempt < maxAttempts; attempt++ {
			username, gerr := GenerateUniqueUsername(tx, profile.Username, emailLocal, profile.DisplayName)
			if gerr != nil {
				return nil, fmt.Errorf("generate username: %w", gerr)
			}
			if _, serr := tx.Exec(`SAVEPOINT oauth_user_insert`); serr != nil {
				return nil, fmt.Errorf("savepoint: %w", serr)
			}
			// OAuth providers authenticate the account, so the email (when
			// present) is treated as verified — email_verified_at is stamped on
			// create so OAuth users never see the verify-email prompt.
			err = tx.QueryRow(`
				INSERT INTO users (id, email, display_name, username, email_verified_at, created_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
				RETURNING id, email, password_hash, display_name, username, created_at
			`, newID, email, profile.DisplayName, username).
				Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt)
			if err == nil {
				if _, rerr := tx.Exec(`RELEASE SAVEPOINT oauth_user_insert`); rerr != nil {
					return nil, fmt.Errorf("release savepoint: %w", rerr)
				}
				break
			}
			if !isUniqueViolation(err, "idx_users_username") {
				break
			}
			// Username collided with a concurrent signup; roll back to the
			// savepoint and regenerate.
			if _, rerr := tx.Exec(`ROLLBACK TO SAVEPOINT oauth_user_insert`); rerr != nil {
				return nil, fmt.Errorf("rollback savepoint: %w", rerr)
			}
		}
	} else {
		// Existing user: refresh the display name but keep their username. Also
		// backfill email_verified_at for accounts that linked OAuth after an
		// unverified email/password signup (or predate this behaviour); COALESCE
		// preserves an already-set verification timestamp.
		err = tx.QueryRow(`
			UPDATE users SET display_name = $1,
			                 email_verified_at = COALESCE(email_verified_at, NOW())
			WHERE id = $2
			RETURNING id, email, password_hash, display_name, username, created_at
		`, profile.DisplayName, userID).
			Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Username, &user.CreatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	var email sql.NullString
	if profile.Email != "" {
		email = sql.NullString{String: profile.Email, Valid: true}
	}
	var avatar sql.NullString
	if profile.AvatarURL != "" {
		avatar = sql.NullString{String: profile.AvatarURL, Valid: true}
	}
	_, err = tx.Exec(`
		INSERT INTO user_providers (user_id, provider, provider_id, email, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_id) DO UPDATE
			SET email = EXCLUDED.email,
			    avatar_url = EXCLUDED.avatar_url
	`, user.ID, profile.Provider, profile.ProviderUserID, email, avatar)
	if err != nil {
		return nil, fmt.Errorf("upsert provider: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &user, nil
}
