package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"
)

type DependencyCheck func(context.Context) error

type HealthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies"`
}

func HealthHandler(service string, checks map[string]DependencyCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		statusCode := http.StatusOK
		deps := make(map[string]string, len(checks))
		for name, check := range checks {
			if err := check(ctx); err != nil {
				deps[name] = "unreachable"
				statusCode = http.StatusServiceUnavailable
				continue
			}
			deps[name] = "ok"
		}

		status := "ok"
		if statusCode != http.StatusOK {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: status, Service: service, Dependencies: deps})
	}
}

func PostgresCheck(databaseURL string) DependencyCheck {
	return tcpURLCheck(databaseURL)
}

func RedisCheck(redisURL string) DependencyCheck {
	return tcpURLCheck(redisURL)
}

func tcpURLCheck(rawURL string) DependencyCheck {
	return func(ctx context.Context) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}

		dialer := net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}

func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
