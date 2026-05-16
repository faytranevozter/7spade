package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	AuthProvider string
	ProviderID   string
	CreatedAt    time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateUser inserts a new user into the database
func CreateUser(db *sql.DB, email, passwordHash, displayName string) (*User, error) {
	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}

	query := `
		INSERT INTO users (id, email, password_hash, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, display_name, auth_provider, provider_id, created_at
	`

	err := db.QueryRow(query, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AuthProvider, &user.ProviderID, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	user := &User{}
	query := `
		SELECT id, email, password_hash, display_name, auth_provider, provider_id, created_at
		FROM users
		WHERE email = $1
	`

	err := db.QueryRow(query, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AuthProvider, &user.ProviderID, &user.CreatedAt)

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
		SELECT id, email, password_hash, display_name, auth_provider, provider_id, created_at
		FROM users
		WHERE id = $1
	`

	err := db.QueryRow(query, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AuthProvider, &user.ProviderID, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// UpsertTelegramUser creates or updates the user linked to a Telegram account.
func UpsertTelegramUser(db *sql.DB, telegramID int64, displayName string) (*User, error) {
	user := &User{}
	providerID := fmt.Sprintf("%d", telegramID)
	query := `
		INSERT INTO users (id, email, password_hash, display_name, auth_provider, provider_id, created_at)
		VALUES ($1, $2, '', $3, 'telegram', $4, $5)
		ON CONFLICT (auth_provider, provider_id)
		DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id, email, password_hash, display_name, auth_provider, provider_id, created_at
	`

	err := db.QueryRow(query, uuid.New(), "telegram:"+providerID, displayName, providerID, time.Now()).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.AuthProvider, &user.ProviderID, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert Telegram user: %w", err)
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
