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
	// conn is the player's live socket, or nil for a remote player served by
	// an edge replica (and briefly across reconnects). Guarded by mu: send and
	// the heartbeat read and write it under mu, and the (re)join assignments in
	// seatLocked / addLobbyPlayerLocked take mu while the caller holds room.mu.
	// room is guarded the same way (send snapshots both under mu).
	conn         *session.Connection
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
	conn        *session.Connection
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
	if s == nil || s.conn == nil {
		return
	}
	if err := s.conn.Send(message); err != nil {
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
	// Hold mu across the conn snapshot and the write so a concurrent reconnect
	// (seatLocked / addLobbyPlayerLocked assign conn under room.mu + mu) can't
	// swap the socket in between and send this frame to a stale conn.
	player.mu.Lock()
	conn := player.conn
	room := player.room
	if conn == nil {
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
	err := conn.Send(message)
	player.mu.Unlock()
	if err != nil {
		// Close outside the lock; the read loop observes the dead conn and runs
		// the normal disconnect handling.
		_ = conn.Close()
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
