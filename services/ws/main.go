package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/relay"
	"github.com/faytranevozter/7spade/services/ws/store"
)

type dependencyCheck func(context.Context) error

type healthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Dependencies map[string]string `json:"dependencies"`
}

func main() {
	cfg := LoadConfig()

	// Live game state is persisted to Redis as room snapshots so rooms survive
	// a WS restart. Redis is required: fail fast if it can't be reached.
	redisClient := newRedisClientFromURL(cfg.RedisURL)
	stateStore := newRedisStateStore(store.New(redisClient, store.DefaultTTL))

	// The dedicated WS Redis backs the cross-replica relay (pub/sub + owner
	// leases). It falls back to REDIS_URL (same client) when WS_REDIS_URL is
	// unset, so single-Redis / single-replica deploys need no extra config.
	wsRedisClient := redisClient
	if cfg.WSRedisURL != "" && cfg.WSRedisURL != cfg.RedisURL {
		wsRedisClient = newRedisClientFromURL(cfg.WSRedisURL)
	}
	replicaID := newReplicaID()
	broker := relay.NewBroker(wsRedisClient)
	leases := relay.NewLeaseManager(wsRedisClient, replicaID, relay.DefaultLeaseTTL)
	log.Printf("WS replica id %s", replicaID)

	gameServer := NewGameServerFromConfig(cfg, stateStore)
	gameServer.attachRelay(replicaID, broker, leases)
	// Presence (friends feature) shares the same Redis client. Optional: if
	// unset, presence reads simply report everyone offline.
	gameServer.presence = store.NewPresence(redisClient, store.PresenceTTL)
	mux := gameServer.routes(map[string]dependencyCheck{
		"postgres": postgresCheck(cfg.DatabaseURL),
		"redis":    redisCheck(cfg.RedisURL),
	})

	// Periodically reconcile live room presence with the API so orphaned
	// 'waiting' rooms (a DB membership whose player never connected over WS)
	// don't linger in the public lobby. No-op when API_URL is unset.
	go gameServer.StartRoomReconciler(context.Background())

	log.Printf("WS service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

// newReplicaID returns a stable-per-process identifier used for owner leases.
// Prefer the container hostname (Swarm task slot / Compose replica) so logs are
// readable; fall back to a random hex id.
func newReplicaID() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		var suffix [4]byte
		if _, err := rand.Read(suffix[:]); err == nil {
			return host + "-" + hex.EncodeToString(suffix[:])
		}
		return host
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "replica-unknown"
	}
	return hex.EncodeToString(id[:])
}

// newRedisClientFromURL connects to Redis from REDIS_URL. Redis is required for
// the WS service: a parse or connectivity failure is fatal.
func newRedisClientFromURL(redisURL string) *redis.Client {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to Redis: %v", err)
	}
	return client
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
