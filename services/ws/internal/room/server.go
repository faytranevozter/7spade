package room

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"

	"github.com/faytranevozter/7spade/services/ws/game"
)

type Manager struct {
	accessChecker       playerAccessChecker
	sessions            session.Runtime
	edges               session.Connections
	rooms               map[string]*room
	store               stateStore
	gameHistory         gameHistoryStore
	statusUpdater       roomStatusUpdater
	memberRemover       roomMemberRemover
	roomSettings        roomSettingsStore
	applicationControls interface{ Enabled(string) bool }
	presence            presenceWriter
	turnTimerDuration   time.Duration
	lobbyLeaveGrace     time.Duration
	rematchWindow       time.Duration
	wsPingEvery         time.Duration
	wsPongWait          time.Duration
	mu                  sync.Mutex

	// Optional cluster capabilities. A nil or disabled cluster keeps the manager
	// in single-replica mode.
	cluster Cluster
	edge    Edge
}

// AttachCluster wires cluster and edge capabilities assembled by app. It is
// optional for single-replica operation.
func (server *Manager) AttachCluster(runtime Cluster, edge Edge) {
	server.cluster = runtime
	server.edge = edge
}

// Shutdown stops the attached cluster runtime during process shutdown and tests.
func (server *Manager) Shutdown() {
	if server.cluster != nil {
		server.cluster.Stop()
	}
}

// relayEnabled reports whether cross-replica coordination is active.
func (server *Manager) relayEnabled() bool {
	return server.cluster != nil && server.cluster.Enabled()
}

// presenceWriter marks users online/offline in a shared store (Redis) so the
// API can report who is currently connected. nil disables presence (tests /
// no-Redis); all calls are guarded by a nil check.
type presenceWriter interface {
	Online(ctx context.Context, userID, roomID string) error
	Offline(ctx context.Context, userID string) error
}

type room struct {
	id                  string
	players             []*player
	state               game.GameState
	botDifficulty       game.BotDifficulty
	practiceMode        bool
	gameConfig          game.GameConfig
	store               stateStore
	gameHistory         gameHistoryStore
	statusUpdater       roomStatusUpdater
	memberRemover       roomMemberRemover
	phase               roomPhase
	started             bool
	startedAt           time.Time
	turnTimerDuration   time.Duration
	lobbyLeaveGrace     time.Duration
	turnExpiresAt       time.Time
	turnTimer           *time.Timer
	turnTimerToken      int
	rematchVotes        map[int]bool
	rematchWindow       time.Duration
	wsPingEvery         time.Duration
	wsPongWait          time.Duration
	accessChecker       playerAccessChecker
	sessions            session.Runtime
	applicationControls interface{ Enabled(string) bool }
	rematchExpiresAt    time.Time
	rematchTimer        *time.Timer
	rematchTimerToken   int
	kickedSubs          map[string]bool
	spectators          []*spectator
	gameDeltas          map[string]playerDelta
	savedGameID         string

	// Replay recording: the hands dealt at the start of the current game and
	// the ordered log of moves applied since the deal. Reset on every deal and
	// persisted in the room snapshot so an in-progress replay survives a WS
	// restart. Shipped to the API on game over.
	initialHands [][]game.Card
	moves        []recordedMove

	// snapVersion is a monotonically-increasing epoch stamped onto every
	// persisted snapshot so the store can drop out-of-order writes (a
	// delayed SaveRoom landing after a DeleteRoom).
	snapVersion     int64
	snapshotSavedAt time.Time

	mu sync.Mutex

	// relay is the room-facing ownership and remote-delivery capability. Nil
	// denotes single-replica mode; local delivery goes through session.Runtime.
	relay Relay

	// teardown removes an empty single-replica room from the manager inventory.
	// Cluster ownership cleanup is managed by the cluster runtime.
	teardown func()
}

// roomSettingsStore supplies persisted room configuration from the API. The WS
// service owns live gameplay, but room creation options are stored by the API.
type roomSettingsStore interface {
	GetRoomSettings(roomID, token string) (roomSettings, error)
}

func normalizeBotDifficulty(value string) game.BotDifficulty {
	switch game.BotDifficulty(strings.ToLower(strings.TrimSpace(value))) {
	case game.BotEasy:
		return game.BotEasy
	case game.BotMedium:
		return game.BotMedium
	case game.BotHard:
		return game.BotHard
	default:
		return game.BotMedium
	}
}

func gameConfigFromSettings(settings roomSettings) game.GameConfig {
	cfg := game.DefaultConfig()
	if settings.MaxPlayers > 0 {
		cfg.PlayerCount = settings.MaxPlayers
	}
	if settings.DeckCount > 0 {
		cfg.DeckCount = settings.DeckCount
	}
	if settings.ScoringMode != "" {
		cfg.ScoringMode = game.ScoringMode(settings.ScoringMode)
	}
	if settings.CustomScores != nil {
		cfg.CustomScores = settings.CustomScores
	}
	if settings.TeamMode != "" {
		cfg.TeamMode = game.TeamMode(settings.TeamMode)
	}
	return cfg
}

func (server *Manager) nextSpectatorID() string {
	n := spectatorIDCounter.Add(1)
	if server.cluster != nil && server.cluster.ID() != "" {
		return server.cluster.ID() + "-spec-" + strconv.FormatUint(n, 10)
	}
	return "spec-" + strconv.FormatUint(n, 10)
}

func (server *Manager) controlEnabled(key string) bool {
	return server.applicationControls == nil || server.applicationControls.Enabled(key)
}

func (room *room) controlEnabled(key string) bool {
	return room.applicationControls == nil || room.applicationControls.Enabled(key)
}
