package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt with cost 12
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedBytes), nil
}

// ComparePassword compares a plain password with a hashed password
func ComparePassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
