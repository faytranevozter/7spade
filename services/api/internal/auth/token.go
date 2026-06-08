package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// GenerateRefreshToken returns a cryptographically random URL-safe token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashRefreshToken returns the SHA-256 base64url hash of a refresh token.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(sum[:])
}

// GenerateURLToken returns a cryptographically random URL-safe token suitable
// for emailed links (password reset, email verification). Same entropy as a
// refresh token; kept as its own name so intent is clear at call sites.
func GenerateURLToken() (string, error) {
	return GenerateRefreshToken()
}

// HashToken returns the SHA-256 base64url hash of an arbitrary token. The raw
// token travels in the email link; only this hash is persisted (in Redis), so a
// store leak cannot be replayed.
func HashToken(token string) string {
	return HashRefreshToken(token)
}
