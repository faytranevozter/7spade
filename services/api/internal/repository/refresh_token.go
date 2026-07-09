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

func RevokeRefreshToken(db *sql.DB, tokenHash string) error {
	if _, err := db.Exec(`DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RotateRefreshToken atomically consumes the presented refresh token and
// returns the owning user. The DELETE ... RETURNING is a single statement, so
// only one concurrent refresh request can succeed for a given token — the rest
// observe zero deleted rows and must reject. This prevents the old
// validate-then-delete-then-insert flow from minting multiple new tokens when
// the same token is refreshed concurrently (e.g. duplicate client requests).
func RotateRefreshToken(db *sql.DB, tokenHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := db.QueryRow(`DELETE FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW() RETURNING user_id`, tokenHash).Scan(&userID)
	if err == sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("refresh token already used or invalid")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	return userID, nil
}

// RevokeAllRefreshTokensForUser deletes every refresh token for a user, forcing
// re-login on all sessions. Used after a password reset.
func RevokeAllRefreshTokensForUser(db *sql.DB, userID uuid.UUID) error {
	if _, err := db.Exec(`DELETE FROM refresh_tokens WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return nil
}
