package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is bcrypt's hard input limit. Validate before hashing so
// handlers return a client error instead of surfacing bcrypt's internal error.
const MaxPasswordBytes = 72

// HashPassword hashes a plain password with bcrypt (cost 12).
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// ComparePassword returns nil when password matches the bcrypt hash.
func ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
