package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/gin-gonic/gin"
)

func newTestRedis(t *testing.T) *cache.RedisClient {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client, err := cache.New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("cache.New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRateLimitAllowsUnderLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := newTestRedis(t)

	r := gin.New()
	r.GET("/ping", RateLimit(rdb, "general", 5, time.Minute, KeyByIP), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimitDeniesWithRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := newTestRedis(t)

	r := gin.New()
	r.GET("/ping", RateLimit(rdb, "auth", 2, time.Minute, KeyByIP), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	retry := w.Header().Get("Retry-After")
	if retry == "" {
		t.Fatal("expected Retry-After header")
	}
	sec, err := strconv.Atoi(retry)
	if err != nil || sec < 1 {
		t.Fatalf("Retry-After = %q, want positive int", retry)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error == "" {
		t.Fatal("expected error message in body")
	}
}

func TestRateLimitSameIPSharesAuthBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := newTestRedis(t)

	r := gin.New()
	r.POST("/login", RateLimit(rdb, "auth", 1, time.Minute, KeyByIP), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodPost, "/login", nil)
	req1.RemoteAddr = "10.0.0.3:1111"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first: status = %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/login", nil)
	req2.RemoteAddr = "10.0.0.3:2222"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second same IP: status = %d, want 429", w2.Code)
	}
}

func TestRateLimitDifferentUsersDoNotShareSocialBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rdb := newTestRedis(t)

	r := gin.New()
	r.GET("/users/search", func(c *gin.Context) {
		// Simulate RequireAuth having set claims.
		sub := c.GetHeader("X-Test-Sub")
		c.Set(ClaimsKey, &auth.Claims{Sub: sub})
		c.Next()
	}, RateLimit(rdb, "social", 1, time.Minute, KeyByUser), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	reqA := httptest.NewRequest(http.MethodGet, "/users/search", nil)
	reqA.Header.Set("X-Test-Sub", "user-a")
	reqA.RemoteAddr = "10.0.0.4:1"
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Fatalf("user-a: status = %d", wA.Code)
	}

	reqB := httptest.NewRequest(http.MethodGet, "/users/search", nil)
	reqB.Header.Set("X-Test-Sub", "user-b")
	reqB.RemoteAddr = "10.0.0.4:1"
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	if wB.Code != http.StatusOK {
		t.Fatalf("user-b should not share user-a bucket: status = %d", wB.Code)
	}
}

type failOpenLimiter struct{}

func (failOpenLimiter) AllowRate(ctx context.Context, scope, subject string, limit int, window time.Duration) (cache.RateResult, error) {
	return cache.RateResult{Allowed: true}, errors.New("redis down")
}

func TestRateLimitFailOpenOnRedisError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/ping", RateLimit(failOpenLimiter{}, "general", 1, time.Minute, KeyByIP), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Even after many requests, fail-open must not 429.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200 (fail-open)", i+1, w.Code)
		}
	}
}

func TestRateLimitNilLimiterIsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ping", RateLimit(nil, "general", 1, time.Minute, KeyByIP), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}
