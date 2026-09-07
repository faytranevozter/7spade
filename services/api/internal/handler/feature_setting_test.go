package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDisabledApplicationControlsRejectNewActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	disabled := func(_ *sql.DB, _ string) (bool, error) { return false, nil }
	tests := []struct {
		name    string
		path    string
		body    string
		handler gin.HandlerFunc
		message string
	}{
		{"guest access", "/guest", `{"display_name":"Guest"}`, AuthHandler{FeatureEnabled: disabled}.Guest, "Guest access is temporarily unavailable"},
		{"registration", "/register", `{}`, AuthHandler{FeatureEnabled: disabled}.Register, "New registrations are temporarily unavailable"},
		{"room creation", "/rooms", `{}`, RoomHandler{FeatureEnabled: disabled}.Create, "Room creation is temporarily unavailable"},
		{"quick play", "/rooms/quick-play", ``, RoomHandler{FeatureEnabled: disabled}.QuickPlay, "Quick Play is temporarily unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.POST(tt.path, tt.handler)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body)))
			if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), tt.message) || !strings.Contains(response.Body.String(), `"code":"feature_disabled"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestApplicationControlLookupFailureFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	failing := func(_ *sql.DB, _ string) (bool, error) { return false, errors.New("database unavailable") }
	r := gin.New()
	r.POST("/guest", AuthHandler{FeatureEnabled: failing}.Guest)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/guest", strings.NewReader(`{"display_name":"Guest"}`)))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}
