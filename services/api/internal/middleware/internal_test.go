package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newInternalRouter(secret string) *gin.Engine {
	r := gin.New()
	g := r.Group("/internal")
	g.Use(RequireInternalSecret(secret))
	g.POST("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func TestRequireInternalSecretDisabledWhenEmpty(t *testing.T) {
	r := newInternalRouter("")
	w := httptest.NewRecorder()
	// No header sent; with no configured secret the guard is a no-op.
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/internal/ping", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRequireInternalSecretAcceptsMatchingHeader(t *testing.T) {
	r := newInternalRouter("s3cret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	req.Header.Set(InternalSecretHeader, "s3cret")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRequireInternalSecretRejectsWrongOrMissingHeader(t *testing.T) {
	cases := map[string]string{
		"missing": "",
		"wrong":   "nope",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			r := newInternalRouter("s3cret")
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
			if header != "" {
				req.Header.Set(InternalSecretHeader, header)
			}
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}
