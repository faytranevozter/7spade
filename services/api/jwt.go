package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents the JWT claims for both guest and registered users
type Claims struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	IsGuest     bool   `json:"is_guest"`
	jwt.RegisteredClaims
}

// For backwards compatibility
type GuestClaims = Claims

// GenerateGuestToken creates a JWT for a guest user
// The token is valid for 7 days and contains:
// - sub: random UUID
// - display_name: provided by the user
// - is_guest: true
// - exp: current time + 7 days
func GenerateGuestToken(displayName string, jwtSecret string) (string, error) {
	if displayName == "" {
		return "", fmt.Errorf("display name cannot be empty")
	}

	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	claims := GuestClaims{
		Sub:         uuid.New().String(),
		DisplayName: displayName,
		IsGuest:     true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// GenerateUserToken creates a JWT for a registered user
// The token is valid for 7 days and contains:
// - sub: user UUID
// - display_name: user's display name
// - is_guest: false
// - exp: current time + 7 days
func GenerateUserToken(userID string, displayName string, jwtSecret string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user ID cannot be empty")
	}
	if displayName == "" {
		return "", fmt.Errorf("display name cannot be empty")
	}

	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	claims := Claims{
		Sub:         userID,
		DisplayName: displayName,
		IsGuest:     false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// ParseGuestToken parses and validates a JWT token
func ParseGuestToken(tokenString string, jwtSecret string) (*GuestClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &GuestClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*GuestClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
