package middleware

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/gin-gonic/gin"
)

// RateLimiter is the Redis-backed fixed-window check used by RateLimit middleware.
type RateLimiter interface {
	AllowRate(ctx context.Context, scope, subject string, limit int, window time.Duration) (cache.RateResult, error)
}

// KeyFunc resolves the rate-limit identity for a request (e.g. IP or user id).
type KeyFunc func(c *gin.Context) string

// KeyByIP rates unauthenticated / auth-sensitive routes by client IP.
func KeyByIP(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		return "unknown"
	}
	return ip
}

// KeyByUser rates authenticated routes by JWT sub, falling back to IP.
func KeyByUser(c *gin.Context) string {
	if claims, ok := ClaimsFromContext(c); ok && claims.Sub != "" {
		return claims.Sub
	}
	return KeyByIP(c)
}

// RateLimit returns Gin middleware that enforces limit actions per window for
// the given scope. On deny it responds 429 with Retry-After and aborts.
// When limiter is nil or limit <= 0 the middleware is a no-op (fail-open).
func RateLimit(limiter RateLimiter, scope string, limit int, window time.Duration, keyFn KeyFunc) gin.HandlerFunc {
	if keyFn == nil {
		keyFn = KeyByIP
	}
	if window <= 0 {
		window = time.Minute
	}
	return func(c *gin.Context) {
		if limiter == nil || limit <= 0 {
			c.Next()
			return
		}
		identity := keyFn(c)
		if identity == "" {
			identity = "unknown"
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		res, err := limiter.AllowRate(ctx, scope, identity, limit, window)
		if err != nil {
			log.Printf("ratelimit: %s check failed (fail-open): %v", scope, err)
		}
		if res.Allowed {
			c.Next()
			return
		}

		retrySec := int(res.RetryAfter.Round(time.Second) / time.Second)
		if retrySec < 1 {
			retrySec = 1
		}
		log.Printf("ratelimit: denied scope=%s identity=%s path=%s limit=%d retry_after=%ds",
			scope, identity, c.FullPath(), limit, retrySec)
		c.Header("Retry-After", strconv.Itoa(retrySec))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please wait"})
	}
}
