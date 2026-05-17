package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        sql.NullString
	PasswordHash sql.NullString
	DisplayName  string
	CreatedAt    time.Time
}

type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	AvatarURL      string
}

func CreateUser(db *sql.DB, email, passwordHash, displayName string) (*User, error) {
	user := &User{ID: uuid.New(), Email: sql.NullString{String: email, Valid: true}, PasswordHash: sql.NullString{String: passwordHash, Valid: true}, DisplayName: displayName, CreatedAt: time.Now()}
	err := db.QueryRow(`
		INSERT INTO users (id, email, password_hash, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, display_name, created_at
	`, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`SELECT id, email, password_hash, display_name, created_at FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
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
	err := db.QueryRow(`SELECT id, email, password_hash, display_name, created_at FROM users WHERE id = $1`, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
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
		err = tx.QueryRow(`
			INSERT INTO users (id, email, display_name, created_at)
			VALUES ($1, $2, $3, NOW())
			RETURNING id, email, password_hash, display_name, created_at
		`, uuid.New(), email, profile.DisplayName).
			Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
	} else {
		err = tx.QueryRow(`
			UPDATE users SET display_name = $1 WHERE id = $2
			RETURNING id, email, password_hash, display_name, created_at
		`, profile.DisplayName, userID).
			Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
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
