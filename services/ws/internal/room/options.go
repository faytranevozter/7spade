package room

import (
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

type playerAccessChecker interface{ CheckAccess(string) error }

// Dependencies are capabilities consumed by the room runtime. Concrete HTTP,
// Redis, and presence adapters are assembled by app, never constructed here.
type Dependencies struct {
	Sessions  session.Runtime
	Edges     session.Connections
	Snapshots stateStore
	History   gameHistoryStore
	Status    roomStatusUpdater
	Members   roomMemberRemover
	Settings  roomSettingsStore
	Access    playerAccessChecker
	Controls  interface{ Enabled(string) bool }
	Presence  presenceWriter
}

type Options struct {
	TurnDuration time.Duration
}

// New creates the authoritative room manager. Room state, seats, timer tokens,
// and locks remain private; callers submit sessions rather than mutating rooms.
func New(deps Dependencies, options Options) *Manager {
	duration := options.TurnDuration
	if duration <= 0 {
		duration = 60 * time.Second
	}
	return &Manager{
		rooms: map[string]*room{}, sessions: deps.Sessions, edges: deps.Edges, store: deps.Snapshots, gameHistory: deps.History,
		statusUpdater: deps.Status, memberRemover: deps.Members,
		roomSettings:  deps.Settings,
		accessChecker: deps.Access, applicationControls: deps.Controls, presence: deps.Presence,
		turnTimerDuration: duration, lobbyLeaveGrace: defaultLobbyLeaveGrace,
		rematchWindow: defaultRematchWindow, wsPingEvery: defaultWebSocketPingEvery,
		wsPongWait: defaultWebSocketPongWait,
	}
}
