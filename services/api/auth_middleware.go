package main

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a private type for storing values in context
type contextKey string

const claimsContextKey contextKey = "claims"

// extractBearerToken pulls the bearer token out of an Authorization header.
// Returns the token string and a boolean indicating whether it was found.
func extractBearerToken(authHeader string) (string, bool) {
	if authHeader == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}
	token := strings.TrimSpace(authHeader[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// requireAuth wraps a handler, validating the bearer JWT and injecting claims
// into the request context. Responds 401 if the token is missing or invalid.
func requireAuth(jwtSecret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		claims, err := ParseGuestToken(token, jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// claimsFromContext retrieves the JWT claims attached by requireAuth.
func claimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok
}
