package room

import (
	"log"
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/session"

	"github.com/faytranevozter/7spade/services/ws/relay"
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
	// inboundAt tracks recent inbound message times for flood protection.
	inboundAt []time.Time
	mu        sync.Mutex

	room *room
}

// Per-connection inbound flood guard. Limits junk/spam without blocking normal
// play_card cadence (turn timer + engine already gate legal turns).
const (
	inboundFloodWindow = 10 * time.Second
	inboundFloodLimit  = 40
	inboundFloodClose  = 80
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

// allowInbound records a message and reports whether it should be handled.
// closeConn is true when the connection should be dropped for sustained abuse.
func (p *player) allowInbound() (ok bool, closeConn bool) {
	if p == nil {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-inboundFloodWindow)
	kept := p.inboundAt[:0]
	for _, t := range p.inboundAt {
		if !t.Before(cutoff) {
			kept = append(kept, t)
		}
	}
	p.inboundAt = append(kept, now)
	n := len(p.inboundAt)
	if n > inboundFloodClose {
		return false, true
	}
	if n > inboundFloodLimit {
		return false, false
	}
	return true, false
}

// spectator is a read-only viewer attached to a room. It holds a connection but
// no seat: spectators never enter room.players, never affect can_start / turn
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
	delivery    session.Delivery
	lastEmoteAt time.Time
	inboundAt   []time.Time
	mu          sync.Mutex
}

func (s *spectator) allowInbound() (ok bool, closeConn bool) {
	if s == nil {
		return false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-inboundFloodWindow)
	kept := s.inboundAt[:0]
	for _, t := range s.inboundAt {
		if !t.Before(cutoff) {
			kept = append(kept, t)
		}
	}
	s.inboundAt = append(kept, now)
	n := len(s.inboundAt)
	if n > inboundFloodClose {
		return false, true
	}
	if n > inboundFloodLimit {
		return false, false
	}
	return true, false
}

// send writes a message to the spectator's socket, guarded by its own mutex so
// concurrent broadcasts don't interleave frames.
func (s *spectator) send(message map[string]any) {
	if s == nil || s.sessionID == "" || s.delivery == nil {
		return
	}
	if err := s.delivery.Send(s.sessionID, message); err != nil {
		log.Printf("write spectator message: %v", err)
	}
}

// Send satisfies relay.Conn so the edge registry can fan an owner-published
// envelope out to this spectator's local socket.
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
		// Remote player: socket lives on an edge replica. Route via the relay so
		// the edge delivers it locally. (Only reached when this replica owns the
		// room under an active relay; in single-process mode conn is always
		// non-nil for a connected player.) The publish runs without holding mu:
		// a slow broker must not block heartbeats and other sends to this player.
		if room != nil {
			room.publishEnvelope(relay.Target{Kind: relay.TargetSub, Sub: player.sub}, message)
		}
		return
	}
	var err error
	if room != nil && room.delivery != nil {
		err = room.delivery.Send(sessionID, message)
	}
	player.mu.Unlock()
	if err != nil {
		// Close through the injected delivery registry. The read loop observes the
		// closure and applies the normal disconnect handling.
		if room != nil && room.delivery != nil {
			room.delivery.Close(sessionID)
		}
		log.Printf("write websocket message: %v", err)
	}
}

// Send satisfies relay.Conn so the edge registry can fan an owner-published
// envelope out to this player's local socket. It is the same write path as
// send; the distinct name keeps the relay interface explicit.
func (player *player) Send(message map[string]any) { player.send(message) }

func (player *player) sendError(message string) {
	player.send(errorMessage(message))
}
