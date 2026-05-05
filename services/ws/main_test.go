package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandlerReportsDependencyStatus(t *testing.T) {
	handler := healthHandler("ws", map[string]dependencyCheck{
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
	if response.Status != "ok" || response.Service != "ws" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Dependencies["postgres"] != "ok" || response.Dependencies["redis"] != "ok" {
		t.Fatalf("unexpected dependencies: %+v", response.Dependencies)
	}
}

func TestHealthHandlerReturnsUnavailableWhenDependencyFails(t *testing.T) {
	handler := healthHandler("ws", map[string]dependencyCheck{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return errors.New("down") },
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
	if response.Status != "degraded" || response.Dependencies["redis"] != "unreachable" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
