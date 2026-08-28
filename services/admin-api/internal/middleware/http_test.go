package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSPreflightAllowsAdminMutationMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS("http://localhost:5174"))

	for _, method := range []string{http.MethodPatch, http.MethodPut} {
		req := httptest.NewRequest(http.MethodOptions, "/users/id/display-name", nil)
		req.Header.Set("Origin", "http://localhost:5174")
		req.Header.Set("Access-Control-Request-Method", method)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, req)

		if response.Code != http.StatusNoContent {
			t.Fatalf("%s preflight status = %d, want %d", method, response.Code, http.StatusNoContent)
		}
		if allowed := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(allowed, method) {
			t.Fatalf("%s preflight methods = %q, want %s", method, allowed, method)
		}
	}
}
