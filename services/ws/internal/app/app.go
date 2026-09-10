// Package app is the composition root for the WebSocket executable.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/apiclient"
	"github.com/faytranevozter/7spade/services/ws/internal/cluster"
	"github.com/faytranevozter/7spade/services/ws/internal/cluster/roombridge"
	"github.com/faytranevozter/7spade/services/ws/internal/config"
	"github.com/faytranevozter/7spade/services/ws/internal/httpserver"
	"github.com/faytranevozter/7spade/services/ws/internal/persistence"
	"github.com/faytranevozter/7spade/services/ws/internal/room"
	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/relay"
	"github.com/faytranevozter/7spade/services/ws/store"
	"github.com/redis/go-redis/v9"
)

func Run(ctx context.Context) error {
	cfg := config.LoadConfig()
	redisClient, err := connectRedis(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connect required Redis: %w", err)
	}
	defer redisClient.Close()

	wsRedisClient := redisClient
	if cfg.WSRedisURL != "" && cfg.WSRedisURL != cfg.RedisURL {
		client, connectErr := connectRedis(cfg.WSRedisURL)
		if connectErr != nil {
			log.Printf("WARNING: WS_REDIS_URL %q unreachable (%v); falling back to REDIS_URL", cfg.WSRedisURL, connectErr)
		} else {
			wsRedisClient = client
			defer client.Close()
		}
	}

	apiURL := strings.TrimRight(cfg.APIURL, "/")
	client := &http.Client{Timeout: 5 * time.Second}
	sessions := session.NewRegistry()
	deps := room.Dependencies{Snapshots: persistence.NewRedis(store.New(redisClient, store.DefaultTTL)), Sessions: sessions, Edges: sessions}
	var reconciler session.RoomReconciler
	var access session.AccessChecker
	var controls *apiclient.ApplicationControlsCache
	if apiURL != "" {
		access = &apiclient.APIPlayerAccessChecker{URL: apiURL, Client: client, Secret: cfg.InternalSecret}
		controls = apiclient.NewApplicationControlsCache(apiURL, cfg.InternalSecret)
		deps.History = &apiclient.APIGameHistoryStore{URL: apiURL + "/internal/games", Client: client, Secret: cfg.InternalSecret}
		deps.Status = &apiclient.APIRoomStatusUpdater{URL: apiURL, Client: client, Secret: cfg.InternalSecret}
		deps.Members = &apiclient.APIRoomMemberRemover{URL: apiURL, Client: client, Secret: cfg.InternalSecret}
		reconciler = &apiclient.APIRoomReconciler{URL: apiURL, Client: client, Secret: cfg.InternalSecret}
		deps.Settings = &apiclient.APIRoomSettingsStore{URL: apiURL, Client: client}
		deps.Access = access
		deps.Controls = controls
	}
	deps.Presence = store.NewPresence(redisClient, store.PresenceTTL)

	manager := room.New(deps, room.Options{})
	if controls != nil {
		refreshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := controls.Refresh(refreshCtx); err != nil {
			log.Printf("initial application controls unavailable; controlled WS operations fail closed: %v", err)
		}
		cancel()
		controls.Start(ctx, apiclient.ApplicationControlsRefreshInterval)
	}

	replicaID := newReplicaID()
	broker := relay.NewBroker(wsRedisClient)
	leases := relay.NewLeaseManager(wsRedisClient, replicaID, relay.DefaultLeaseTTL)
	runtime := cluster.NewRuntime(replicaID, broker, leases, relay.NewCoordinator(wsRedisClient, replicaID))
	edge := runtime.NewEdge(sessions, 25*time.Second, 60*time.Second, access, manager.StartEdgePresence)
	manager.AttachCluster(roombridge.New(runtime), edge)
	defer manager.Shutdown()
	go session.StartReconciler(ctx, manager, reconciler, runtime.Coordinator())

	routes := httpserver.Routes(
		map[string]httpserver.DependencyCheck{"postgres": httpserver.PostgresCheck(cfg.DatabaseURL), "redis": httpserver.RedisCheck(cfg.RedisURL)},
		session.Handler(cfg.JWTSecret, access, sessions, session.Admit(manager, sessions)), cfg.InspectionSecret, manager.Inspect,
	)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: httpserver.WithCORS(routes), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("WS replica %s listening on %s", replicaID, server.Addr)
	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func connectRedis(rawURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func newReplicaID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "replica"
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return host
	}
	return host + "-" + hex.EncodeToString(suffix[:])
}
