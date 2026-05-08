package main

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	
	if hash == "" {
		t.Fatal("Hash should not be empty")
	}
	
	if hash == password {
		t.Fatal("Hash should not equal plain password")
	}
}

func TestComparePassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	
	// Test correct password
	if err := ComparePassword(hash, password); err != nil {
		t.Errorf("ComparePassword should succeed with correct password, got error: %v", err)
	}
	
	// Test incorrect password
	if err := ComparePassword(hash, "wrongpassword"); err == nil {
		t.Error("ComparePassword should fail with incorrect password")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	token1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	
	if token1 == "" {
		t.Fatal("Generated token should not be empty")
	}
	
	// Generate another token to ensure they're unique
	token2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}
}

func TestHashRefreshToken(t *testing.T) {
	token := "test-refresh-token"
	
	hash1 := HashRefreshToken(token)
	hash2 := HashRefreshToken(token)
	
	if hash1 == "" {
		t.Fatal("Hash should not be empty")
	}
	
	// Same token should produce same hash (deterministic)
	if hash1 != hash2 {
		t.Error("Same token should produce same hash")
	}
	
	// Different token should produce different hash
	hash3 := HashRefreshToken("different-token")
	if hash1 == hash3 {
		t.Error("Different tokens should produce different hashes")
	}
}
