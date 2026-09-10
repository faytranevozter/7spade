package room

import (
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
		Members: memberRemover, Reconciler: reconciler, Settings: roomSettings,
		Sessions: sessions, Edges: sessions,
	}, Options{TurnDuration: turnTimerDuration}), sessions: sessions, jwtSecret: cfg.JWTSecret, inspectionSecret: cfg.InspectionSecret}
	if apiURL := strings.TrimRight(cfg.APIURL, "/"); apiURL != "" {
		server.applicationControls = newApplicationControlsCache(apiURL, cfg.InternalSecret)
		server.accessChecker = &apiPlayerAccessChecker{URL: apiURL, Client: &http.Client{Timeout: 5 * time.Second}, Secret: cfg.InternalSecret}
	}
	return server
}

type GameServer struct {
	*Manager
	sessions                    *session.Registry
	jwtSecret, inspectionSecret string
}

func (server *GameServer) attachRelay(replicaID string, broker *relay.Broker, leases *relay.LeaseManager, coordinator *relay.Coordinator) {
	server.AttachRelay(replicaID, broker, leases, coordinator)
}

func (server *GameServer) shutdownRelay() { server.Shutdown() }

func (s *GameServer) routes(checks map[string]dependencyCheck) http.Handler {
	return httpserver.Routes(checks, session.Handler(s.jwtSecret, s.accessChecker, s.sessions, s.Serve), s.inspectionSecret, s.Inspect)
}
func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	session.Handler(s.jwtSecret, s.accessChecker, s.sessions, s.Serve)(w, r)
}
func (s *GameServer) handleRoomInspection(w http.ResponseWriter, r *http.Request) {
	httpserver.InspectionHandler(s.inspectionSecret, s.Inspect, false)(w, r)
}
func (s *GameServer) handleHiddenRoomStateInspection(w http.ResponseWriter, r *http.Request) {
	httpserver.InspectionHandler(s.inspectionSecret, s.Inspect, true)(w, r)
}
