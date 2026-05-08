package main

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateGuestToken(t *testing.T) {
	secret := "test-secret"
	displayName := "TestUser"

	token, err := GenerateGuestToken(displayName, secret)
	if err != nil {
		t.Fatalf("GenerateGuestToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestGenerateGuestTokenWithEmptyDisplayName(t *testing.T) {
	secret := "test-secret"
	displayName := ""

	_, err := GenerateGuestToken(displayName, secret)
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}

func TestGeneratedTokenContainsCorrectClaims(t *testing.T) {
	secret := "test-secret"
	displayName := "TestUser"

	tokenString, err := GenerateGuestToken(displayName, secret)
	if err != nil {
		t.Fatalf("GenerateGuestToken failed: %v", err)
	}

	// Parse the token
	claims, err := ParseGuestToken(tokenString, secret)
	if err != nil {
		t.Fatalf("ParseGuestToken failed: %v", err)
	}

	// Verify claims
	if claims.DisplayName != displayName {
		t.Errorf("expected display_name %q, got %q", displayName, claims.DisplayName)
	}

	if !claims.IsGuest {
		t.Error("expected is_guest to be true")
	}

	if claims.Sub == "" {
		t.Error("expected non-empty sub (UUID)")
	}

	// Verify expiration is ~7 days from now
	expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
	actualExpiry := claims.ExpiresAt.Time
	diff := actualExpiry.Sub(expectedExpiry).Abs()
	if diff > 5*time.Second {
		t.Errorf("expected expiry ~7 days from now, got %v (diff: %v)", actualExpiry, diff)
	}
}

func TestParseGuestTokenWithInvalidToken(t *testing.T) {
	secret := "test-secret"
	invalidToken := "invalid.token.here"

	_, err := ParseGuestToken(invalidToken, secret)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestParseGuestTokenWithWrongSecret(t *testing.T) {
	secret := "test-secret"
	wrongSecret := "wrong-secret"
	displayName := "TestUser"

	tokenString, err := GenerateGuestToken(displayName, secret)
	if err != nil {
		t.Fatalf("GenerateGuestToken failed: %v", err)
	}

	_, err = ParseGuestToken(tokenString, wrongSecret)
	if err == nil {
		t.Fatal("expected error when parsing token with wrong secret")
	}
}

func TestParseGuestTokenWithExpiredToken(t *testing.T) {
	secret := "test-secret"
	displayName := "TestUser"

	// Create an expired token
	now := time.Now()
	claims := GuestClaims{
		Sub:         "test-uuid",
		DisplayName: displayName,
		IsGuest:     true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	_, err = ParseGuestToken(tokenString, secret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestGenerateUserToken(t *testing.T) {
	secret := "test-secret"
	userID := "550e8400-e29b-41d4-a716-446655440000"
	displayName := "TestUser"

	token, err := GenerateUserToken(userID, displayName, secret)
	if err != nil {
		t.Fatalf("GenerateUserToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Parse and verify claims
	claims, err := ParseGuestToken(token, secret)
	if err != nil {
		t.Fatalf("ParseGuestToken failed: %v", err)
	}

	if claims.Sub != userID {
		t.Errorf("expected sub %q, got %q", userID, claims.Sub)
	}

	if claims.DisplayName != displayName {
		t.Errorf("expected display_name %q, got %q", displayName, claims.DisplayName)
	}

	if claims.IsGuest {
		t.Error("expected is_guest to be false for registered user")
	}
}

func TestGenerateUserTokenWithEmptyUserID(t *testing.T) {
	secret := "test-secret"
	displayName := "TestUser"

	_, err := GenerateUserToken("", displayName, secret)
	if err == nil {
		t.Fatal("expected error for empty user ID")
	}
}

func TestGenerateUserTokenWithEmptyDisplayName(t *testing.T) {
	secret := "test-secret"
	userID := "550e8400-e29b-41d4-a716-446655440000"

	_, err := GenerateUserToken(userID, "", secret)
	if err == nil {
		t.Fatal("expected error for empty display name")
	}
}
