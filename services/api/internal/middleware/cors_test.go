package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSCredentialedLocalhostOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.GET("/auth/github/url", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"url": "https://github.com/login/oauth/authorize"})
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/github/url", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Cookie", "refresh_token=test")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost origin", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", got)
	}
}

func TestCORSPreflightCredentialedLocalhostOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))

	req := httptest.NewRequest(http.MethodOptions, "/auth/github/url", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost origin", got)
	}
}

func TestCORSRejectsOriginNotInConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.GET("/auth/github/url", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"url": "https://github.com/login/oauth/authorize"})
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/github/url", nil)
	req.Header.Set("Origin", "http://evil.localhost:5173")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
}
