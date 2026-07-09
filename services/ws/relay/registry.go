package relay

import "sync"

// Conn is the minimal behaviour the registry needs from a live socket. Both
// player and spectator connections satisfy it, so the edge can fan an Envelope
// out without depending on the concrete server types. Implementations must be
// safe for concurrent Send (the existing player/spectator types guard writes
// with their own mutex).
type Conn interface {
	Send(payload map[string]any)
}

// localConn is one registered socket on this replica and the routing facts the
// envelope selector matches against. A spectator has spectator=true; players are
// keyed and routed by sub (the edge knows the sub at connect time, before the
// owner assigns a seat).
type localConn struct {
	conn      Conn
	sub       string
	spectator bool
}

// Registry tracks the sockets a replica holds for each room so it can deliver
// owner-published [Envelope]s to exactly the right local recipients. It is the
// edge side of the relay: it contains no game logic, only selector matching.
//
// A replica registers every socket it accepts (player or spectator); when an
// outbound envelope arrives on the room's channel, Deliver routes it by the
// envelope's [Target] to the matching local sockets. The same matching runs
// whether the socket's owner is this replica or another, which is what lets any
// replica serve any of a room's players.
type Registry struct {
	mu    sync.RWMutex
	rooms map[string][]*localConn
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	return &Registry{rooms: map[string][]*localConn{}}
}

// AddPlayer registers a seated player's socket for a room. Re-registering the
// same (room, sub) replaces the previous entry so a reconnect doesn't leave a
// stale socket receiving frames.
func (r *Registry) AddPlayer(roomID, sub string, conn Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := r.rooms[roomID]
	for i, e := range entries {
		if !e.spectator && e.sub == sub {
			entries[i] = &localConn{conn: conn, sub: sub}
			r.rooms[roomID] = entries
			return
		}
	}
	r.rooms[roomID] = append(entries, &localConn{conn: conn, sub: sub})
}

// AddSpectator registers a spectator socket for a room. spectatorID is an
// opaque per-connection key used only for removal.
func (r *Registry) AddSpectator(roomID, spectatorID string, conn Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[roomID] = append(r.rooms[roomID], &localConn{conn: conn, sub: spectatorID, spectator: true})
}

// RemovePlayer drops a player's socket from a room.
func (r *Registry) RemovePlayer(roomID, sub string) {
	r.remove(roomID, sub, false)
}

// RemoveSpectator drops a spectator's socket from a room.
func (r *Registry) RemoveSpectator(roomID, spectatorID string) {
	r.remove(roomID, spectatorID, true)
}

func (r *Registry) remove(roomID, key string, spectator bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := r.rooms[roomID]
	kept := entries[:0]
	for _, e := range entries {
		if e.spectator == spectator && e.sub == key {
			continue
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		delete(r.rooms, roomID)
		return
	}
	r.rooms[roomID] = kept
}

// HasRoom reports whether this replica holds any socket for the room.
func (r *Registry) HasRoom(roomID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.rooms[roomID]) > 0
}

// CountPlayers returns the number of live (non-spectator) sockets this replica
// holds for a given sub in a room. A sub can have several (e.g. two browser
// tabs), so a single edge leave must not mark the seat disconnected while
// another connection for the same player is still alive.
func (r *Registry) CountPlayers(roomID, sub string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, e := range r.rooms[roomID] {
		if !e.spectator && e.sub == sub {
			n++
		}
	}
	return n
}

// Deliver routes one outbound envelope to the local sockets of a room that
// match its target. It performs no I/O beyond the matching connections' Send.
func (r *Registry) Deliver(roomID string, env Envelope) {
	r.mu.RLock()
	entries := append([]*localConn(nil), r.rooms[roomID]...)
	r.mu.RUnlock()
	for _, e := range entries {
		if matches(env.Target, e) {
			e.conn.Send(env.Payload)
		}
	}
}

// matches decides whether a registered socket should receive an envelope.
// Sub/All target only players; Spectators targets every spectator and Spectator
// targets a single spectator by id. This is the single point that enforces "a
// player only receives their own seat view" across replicas.
func matches(t Target, e *localConn) bool {
	switch t.Kind {
	case TargetSpectators:
		return e.spectator
	case TargetSpectator:
		return e.spectator && e.sub == t.Sub
	case TargetAll:
		return !e.spectator
	case TargetSub:
		return !e.spectator && e.sub == t.Sub
	default:
		return false
	}
}
