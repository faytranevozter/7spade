package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func StoreRefreshToken(db *sql.DB, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), userID, tokenHash, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

func ValidateRefreshToken(db *sql.DB, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	var expiresAt time.Time
	err := db.QueryRow(`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = $1`, tokenHash).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("invalid refresh token")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("validate refresh token: %w", err)
	}
	if time.Now().After(expiresAt) {
		return uuid.Nil, fmt.Errorf("refresh token expired")
	}
	return userID, nil
}

func RevokeRefreshToken(db *sql.DB, tokenHash string) error {
	if _, err := db.Exec(`DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllRefreshTokensForUser deletes every refresh token for a user, forcing
// re-login on all sessions. Used after a password reset.
func RevokeAllRefreshTokensForUser(db *sql.DB, userID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return nil
}
