package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faytranevozter/7spade/services/api/internal/config"
)

func TestRouterRegistersMobileTelegramLogin(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	router := NewRouter(&config.Config{}, db, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/mobile/telegram", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestRouterDoesNotRegisterLegacyMobileTelegramRoutes(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	router := NewRouter(&config.Config{}, db, nil)
	for _, route := range []string{
		"/auth/mobile/telegram/start",
		"/auth/mobile/telegram/callback",
		"/auth/mobile/telegram/exchange",
	} {
		t.Run(route, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, route, nil))
			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
			}
		})
	}
}
