package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandlerReportsDependencyStatus(t *testing.T) {
	handler := healthHandler("api", map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || response.Service != "api" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Dependencies["postgres"] != "ok" || response.Dependencies["redis"] != "ok" {
		t.Fatalf("unexpected dependencies: %+v", response.Dependencies)
	}
}

func TestHealthHandlerReturnsUnavailableWhenDependencyFails(t *testing.T) {
	handler := healthHandler("api", map[string]dependencyCheck{
		"postgres": func(context.Context) error { return errors.New("down") },
		"redis":    func(context.Context) error { return nil },
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var response healthResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "degraded" || response.Dependencies["postgres"] != "unreachable" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestGuestHandlerReturnsTokenForValidDisplayName(t *testing.T) {
	secret := "test-secret"
	handler := guestHandler(secret)

	body := bytes.NewBufferString(`{"display_name": "TestUser"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response guestResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify the token is valid
	claims, err := ParseGuestToken(response.Token, secret)
	if err != nil {
		t.Fatalf("failed to parse returned token: %v", err)
	}

	if claims.DisplayName != "TestUser" {
		t.Errorf("expected display_name 'TestUser', got %q", claims.DisplayName)
	}

	if !claims.IsGuest {
		t.Error("expected is_guest to be true")
	}
}

func TestGuestHandlerReturns400ForEmptyDisplayName(t *testing.T) {
	handler := guestHandler("test-secret")

	body := bytes.NewBufferString(`{"display_name": ""}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestGuestHandlerReturns400ForDisplayNameTooLong(t *testing.T) {
	handler := guestHandler("test-secret")

	longName := strings.Repeat("a", 51) // 51 characters
	body := bytes.NewBufferString(`{"display_name": "` + longName + `"}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.Contains(response.Error, "50 characters") {
		t.Errorf("expected error about 50 characters, got %q", response.Error)
	}
}

func TestGuestHandlerReturns400ForInvalidJSON(t *testing.T) {
	handler := guestHandler("test-secret")

	body := bytes.NewBufferString(`invalid json`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestGuestHandlerTrimsWhitespaceFromDisplayName(t *testing.T) {
	secret := "test-secret"
	handler := guestHandler(secret)

	body := bytes.NewBufferString(`{"display_name": "  TestUser  "}`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/guest", body))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response guestResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	claims, err := ParseGuestToken(response.Token, secret)
	if err != nil {
		t.Fatalf("failed to parse returned token: %v", err)
	}

	if claims.DisplayName != "TestUser" {
		t.Errorf("expected trimmed display_name 'TestUser', got %q", claims.DisplayName)
	}
}
