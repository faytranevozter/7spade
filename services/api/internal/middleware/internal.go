package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// InternalSecretHeader carries the shared secret on requests to /internal/*
// endpoints, which are called by the WS service rather than end users.
const InternalSecretHeader = "X-Internal-Secret"

// RequireInternalSecret guards internal service-to-service endpoints with a
// shared secret. When secret is empty the guard is disabled (no enforcement),
// preserving the previous behaviour for deployments that don't set it. When
// set, requests must present a matching X-Internal-Secret header.
func RequireInternalSecret(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.Next()
			return
		}
		provided := c.GetHeader(InternalSecretHeader)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}
