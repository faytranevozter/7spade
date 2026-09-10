package room

import (
	"log"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

type player struct {
	sub         string
	displayName string
	avatar      string
	isGuest     bool
	isBot       bool
	ready       bool
	index       int
	team        int
	// sessionID identifies the local connection in the injected delivery
	// registry. An empty ID denotes a player served by a remote relay edge.
	sessionID    session.ID
	disconnected bool
	leaveTimer   *time.Timer
	leaveToken   int
	lastEmoteAt  time.Time
	mu           sync.Mutex

	room *room
}

const (
	websocketWriteWait = 10 * time.Second
	websocketReadLimit = 32 * 1024
	accessCheckEvery   = 5 * time.Second

	// Heartbeat defaults, overridable per Manager (and inherited per room at
	// creation) so tests can exercise liveness on a fast clock. A ping goes out
	// every pingEvery; if no pong (or any inbound frame) lands within pongWait,
	// the read deadline expires and the connection is dropped.
	defaultWebSocketPongWait  = 60 * time.Second
	defaultWebSocketPingEvery = (defaultWebSocketPongWait * 9) / 10
)

// spectator is a read-only viewer attached to a room through a local session but
// has no seat: spectators never enter room.players, never affect can_start / turn
// order / bot backfill / results / rematch, and are never persisted to the
// room snapshot. Their identity is kept only for logging/debugging.
//
// Spectators may emote (a purely cosmetic social action that never touches game
// state); id uniquely identifies the spectator connection in emote broadcasts
// (so the same sub can spectate from several tabs, and guests don't collide),
// and lastEmoteAt rate-limits those emotes per the spectatorEmoteCooldown.
type spectator struct {
	sub         string
	id          string
	sessionID   session.ID
	sessions    session.Runtime
	lastEmoteAt time.Time
}

// send delegates delivery through session.Runtime, whose session and transport
// path serializes writes.
func (s *spectator) send(message map[string]any) {
	if s == nil || s.sessionID == "" || s.sessions == nil {
		return
	}
	if err := s.sessions.Send(s.sessionID, message); err != nil {
		log.Printf("write spectator message: %v", err)
	}
}

// Send exposes the spectator's normal room-delivery path.
func (s *spectator) Send(message map[string]any) { s.send(message) }

func (player *player) send(message map[string]any) {
	if player == nil {
		return
	}
	// Hold mu across the session ID snapshot so a concurrent reconnect cannot
	// deliver a stale state frame to the connection it replaced.
	player.mu.Lock()
	sessionID := player.sessionID
	room := player.room
	if sessionID == "" {
		player.mu.Unlock()
		// Remote player: its session is held by an edge replica. Route through the
		// Relay capability so the edge delivers locally. The publication runs without holding mu:
		// a slow broker must not block heartbeats and other sends to this player.
		if room != nil {
			room.publishToPlayer(player.sub, message)
		}
		return
	}
	var err error
	if room != nil && room.sessions != nil {
		err = room.sessions.Send(sessionID, message)
	}
	player.mu.Unlock()
	if err != nil {
		// Close through the injected delivery registry. The read loop observes the
		// closure and applies the normal disconnect handling.
		if room != nil && room.sessions != nil {
			room.sessions.Close(sessionID)
		}
		log.Printf("write websocket message: %v", err)
	}
}

// Send exposes the player's normal room-delivery path.
func (player *player) Send(message map[string]any) { player.send(message) }

func (player *player) sendError(message string) {
	player.send(errorMessage(message))
}
