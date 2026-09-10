package room

import (
	"context"
	"github.com/faytranevozter/7spade/services/ws/internal/cluster"
	"github.com/faytranevozter/7spade/services/ws/internal/httpserver"
	"github.com/faytranevozter/7spade/services/ws/internal/session"
	"github.com/faytranevozter/7spade/services/ws/relay"

	"net/http"

	"strings"
	"time"
)

func NewGameServerFromConfig(cfg Config, store stateStore) *GameServer {
	return NewGameServerWithOptions(cfg, store, 60*time.Second)
}

func NewGameServer(jwtSecret string) *GameServer {
	return NewGameServerWithStateStore(jwtSecret, newMemoryStateStore())
}

func NewGameServerWithStateStore(jwtSecret string, store stateStore) *GameServer {
	return NewGameServerWithOptions(Config{JWTSecret: jwtSecret}, store, 60*time.Second)
}

func NewGameServerWithOptions(cfg Config, store stateStore, turnTimerDuration time.Duration) *GameServer {
	historyStore := gameHistoryStore(nil)
	var statusUpdater roomStatusUpdater
	var memberRemover roomMemberRemover
	var reconciler roomReconciler
	var roomSettings roomSettingsStore
	if apiURL := strings.TrimRight(cfg.APIURL, "/"); apiURL != "" {
		historyStore = &apiGameHistoryStore{URL: apiURL + "/internal/games", Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
		statusUpdater = &apiRoomStatusUpdater{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
		memberRemover = &apiRoomMemberRemover{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
		reconciler = &apiRoomReconciler{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
		roomSettings = &apiRoomSettingsStore{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}}
	}
	sessions := session.NewRegistry()
	server := &GameServer{Manager: New(Dependencies{
		Snapshots: store, History: historyStore, Status: statusUpdater,
		Members: memberRemover, Settings: roomSettings,
		Sessions: sessions, Edges: sessions,
	}, Options{TurnDuration: turnTimerDuration}), sessions: sessions, reconciler: reconciler, jwtSecret: cfg.JWTSecret, inspectionSecret: cfg.InspectionSecret}
	if apiURL := strings.TrimRight(cfg.APIURL, "/"); apiURL != "" {
		server.applicationControls = newApplicationControlsCache(apiURL, cfg.InternalSecret)
		server.accessChecker = &apiPlayerAccessChecker{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
	}
	return server
}

type GameServer struct {
	*Manager
	sessions                    *session.Registry
	reconciler                  session.RoomReconciler
	jwtSecret, inspectionSecret string
}

func (server *GameServer) attachRelay(replicaID string, broker *relay.Broker, leases *relay.LeaseManager, coordinator *relay.Coordinator) {
	runtime := cluster.NewRuntime(replicaID, broker, leases, coordinator)
	edge := runtime.NewEdge(server.sessions, server.wsPingEvery, server.wsPongWait, server.accessChecker, server.startPresenceForUser)
	server.AttachCluster(testRoomCluster{runtime}, edge)
}

type testRoomCluster struct{ runtime *cluster.Runtime }

func (c testRoomCluster) Enabled() bool                         { return c.runtime.Enabled() }
func (c testRoomCluster) ID() string                            { return c.runtime.ID() }
func (c testRoomCluster) Context() context.Context              { return c.runtime.Context() }
func (c testRoomCluster) Stop()                                 { c.runtime.Stop() }
func (c testRoomCluster) Acquire(id string) (bool, int64, bool) { return c.runtime.Acquire(id) }
func (c testRoomCluster) IsLocalOwner(id string) bool           { return c.runtime.IsLocalOwner(id) }
func (c testRoomCluster) Inspect(ctx context.Context, id string) (string, int64, error) {
	return c.runtime.Inspect(ctx, id)
}
func (c testRoomCluster) NewRelay(id string) Relay { return c.runtime.NewOwnership(id) }
func (c testRoomCluster) PlayerConnections(roomID, sub string) int {
	return c.runtime.Registry().CountPlayers(roomID, sub)
}
func (c testRoomCluster) Coordinator() Coordinator { return c.runtime.Coordinator() }
func (c testRoomCluster) Promote(handle Relay, token int64, newly bool, callback func(Inbound)) {
	ownership, ok := handle.(*cluster.Ownership)
	if !ok {
		return
	}
	dispatcher := cluster.NewDispatcher(ownership.IsOwner, cluster.InboundHandlers{
		Join: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundJoin, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		Leave: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundLeave, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		Data: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundData, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorJoin: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundSpectatorJoin, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorLeave: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundSpectatorLeave, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
		SpectatorData: func(in cluster.Inbound) {
			callback(Inbound{Kind: InboundSpectatorData, Sub: in.Sub, SpectatorID: in.SpectatorID, Payload: in.Payload})
		},
	})
	ownership.Promote(c.runtime.Context(), token, newly, dispatcher.Handle)
}

func (server *GameServer) shutdownRelay() { server.Shutdown() }

func (s *GameServer) routes(checks map[string]dependencyCheck) http.Handler {
	return httpserver.Routes(checks, session.Handler(s.jwtSecret, s.accessChecker, s.sessions, session.Admit(s, s.sessions)), s.inspectionSecret, s.Inspect)
}
func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	session.Handler(s.jwtSecret, s.accessChecker, s.sessions, session.Admit(s, s.sessions))(w, r)
}
func (s *GameServer) handleRoomInspection(w http.ResponseWriter, r *http.Request) {
	httpserver.InspectionHandler(s.inspectionSecret, s.Inspect, false)(w, r)
}
func (s *GameServer) handleHiddenRoomStateInspection(w http.ResponseWriter, r *http.Request) {
	httpserver.InspectionHandler(s.inspectionSecret, s.Inspect, true)(w, r)
}
