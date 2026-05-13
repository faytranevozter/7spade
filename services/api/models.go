package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID
	Email          string
	PasswordHash   sql.NullString
	DisplayName    string
	Provider       sql.NullString
	ProviderUserID sql.NullString
	AvatarURL      sql.NullString
	CreatedAt      time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateUser inserts a new user into the database (email/password registration)
func CreateUser(db *sql.DB, email, passwordHash, displayName string) (*User, error) {
	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: sql.NullString{String: passwordHash, Valid: true},
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
	`

	err := db.QueryRow(query, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE email = $1
	`

	err := db.QueryRow(query, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(db *sql.DB, id uuid.UUID) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE id = $1
	`

	err := db.QueryRow(query, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// GetUserByProvider returns the user matching (provider, provider_user_id), or nil if not found.
func GetUserByProvider(db *sql.DB, provider, providerUserID string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		FROM users
		WHERE provider = $1 AND provider_user_id = $2
	`

	err := db.QueryRow(query, provider, providerUserID).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by provider: %w", err)
	}

	return user, nil
}

// OAuthProfile holds the normalised user profile information returned by an OAuth provider.
type OAuthProfile struct {
	Provider       string
	ProviderUserID string
	Email          string
	DisplayName    string
	AvatarURL      string
}

// UpsertOAuthUser creates a user for the given OAuth profile, or updates an existing matching
// user (matched first by provider+provider_user_id, then by email). Returns the persisted user.
func UpsertOAuthUser(db *sql.DB, profile OAuthProfile) (*User, error) {
	if profile.Provider == "" || profile.ProviderUserID == "" {
		return nil, fmt.Errorf("provider and provider_user_id are required")
	}
	if profile.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Email
	}

	// Match by provider identity first
	existing, err := GetUserByProvider(db, profile.Provider, profile.ProviderUserID)
	if err != nil {
		return nil, err
	}

	// Fall back to email (e.g. user previously registered with email/password)
	if existing == nil {
		existing, err = GetUserByEmail(db, profile.Email)
		if err != nil {
			return nil, err
		}
	}

	avatar := sql.NullString{String: profile.AvatarURL, Valid: profile.AvatarURL != ""}

	if existing != nil {
		// Update display name, avatar, and link the provider identity if missing
		query := `
			UPDATE users
			SET display_name = $1,
			    avatar_url = $2,
			    provider = $3,
			    provider_user_id = $4
			WHERE id = $5
			RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
		`
		updated := &User{}
		err := db.QueryRow(query, profile.DisplayName, avatar, profile.Provider, profile.ProviderUserID, existing.ID).
			Scan(&updated.ID, &updated.Email, &updated.PasswordHash, &updated.DisplayName, &updated.Provider, &updated.ProviderUserID, &updated.AvatarURL, &updated.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to update oauth user: %w", err)
		}
		return updated, nil
	}

	// Create new OAuth user (no password)
	user := &User{
		ID:             uuid.New(),
		Email:          profile.Email,
		DisplayName:    profile.DisplayName,
		Provider:       sql.NullString{String: profile.Provider, Valid: true},
		ProviderUserID: sql.NullString{String: profile.ProviderUserID, Valid: true},
		AvatarURL:      avatar,
		CreatedAt:      time.Now(),
	}
	insert := `
		INSERT INTO users (id, email, display_name, provider, provider_user_id, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password_hash, display_name, provider, provider_user_id, avatar_url, created_at
	`
	err = db.QueryRow(insert, user.ID, user.Email, user.DisplayName, user.Provider, user.ProviderUserID, user.AvatarURL, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.Provider, &user.ProviderUserID, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert oauth user: %w", err)
	}
	return user, nil
}

// StoreRefreshToken stores a refresh token in the database
func StoreRefreshToken(db *sql.DB, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := db.Exec(query, uuid.New(), userID, tokenHash, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

// ValidateRefreshToken validates a refresh token and returns the user ID
func ValidateRefreshToken(db *sql.DB, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	var expiresAt time.Time

	query := `
		SELECT user_id, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	err := db.QueryRow(query, tokenHash).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("invalid refresh token")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to validate refresh token: %w", err)
	}

	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("refresh token expired")
	}

	return userID, nil
}

// RevokeRefreshToken revokes a refresh token
func RevokeRefreshToken(db *sql.DB, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`
	_, err := db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}
