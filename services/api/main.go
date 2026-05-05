package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type dependencyCheck func(context.Context) error

type healthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("api", map[string]dependencyCheck{
		"postgres": postgresCheck(os.Getenv("DATABASE_URL")),
		"redis":    redisCheck(os.Getenv("REDIS_URL")),
	}))

	log.Printf("API service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(service string, checks map[string]dependencyCheck) http.HandlerFunc {
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
		_ = json.NewEncoder(w).Encode(healthResponse{Status: status, Service: service, Dependencies: deps})
	}
}

func postgresCheck(databaseURL string) dependencyCheck {
	return tcpURLCheck(databaseURL)
}

func redisCheck(redisURL string) dependencyCheck {
	return tcpURLCheck(redisURL)
}

func tcpURLCheck(rawURL string) dependencyCheck {
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

func withCORS(next http.Handler) http.Handler {
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
