package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims holds the JWT payload for both guest and registered users.
type Claims struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	IsGuest     bool   `json:"is_guest"`
	jwt.RegisteredClaims
}

const tokenTTL = 7 * 24 * time.Hour

// GenerateGuestToken creates a 7-day JWT for a guest (no DB row).
func GenerateGuestToken(displayName, secret string) (string, error) {
	if displayName == "" {
		return "", fmt.Errorf("auth: display name cannot be empty")
	}
	now := time.Now()
	claims := Claims{
		Sub:         uuid.New().String(),
		DisplayName: displayName,
		IsGuest:     true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// GenerateUserToken creates a 7-day JWT for a registered user.
func GenerateUserToken(userID, displayName, secret string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("auth: user ID cannot be empty")
	}
	if displayName == "" {
		return "", fmt.Errorf("auth: display name cannot be empty")
	}
	now := time.Now()
	claims := Claims{
		Sub:         userID,
		DisplayName: displayName,
		IsGuest:     false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken parses and validates a JWT, returning its claims.
func ParseToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("auth: invalid token")
}
