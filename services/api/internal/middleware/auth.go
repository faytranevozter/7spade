package middleware

import (
	"net/http"
	"strings"

	"github.com/faytranevozter/7spade/services/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const ClaimsKey = "claims"

func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := ExtractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		claims, err := auth.ParseToken(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) (*auth.Claims, bool) {
	claims, ok := c.Get(ClaimsKey)
	if !ok {
		return nil, false
	}
	typed, ok := claims.(*auth.Claims)
	return typed, ok
}

func ExtractBearerToken(authHeader string) (string, bool) {
	if authHeader == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}
	token := strings.TrimSpace(authHeader[len(prefix):])
	return token, token != ""
}
