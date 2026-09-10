package relay

import "sync"

// Conn is the minimal edge-delivery behaviour the registry needs. Cluster edge
// adapters implement it without exposing concrete session or transport types.
// Implementations must make concurrent Send calls safe.
type Conn interface {
	Send(payload map[string]any)
}

// localConn is one registered socket on this replica and the routing facts the
// envelope selector matches against. A spectator has spectator=true; players are
// keyed and routed by sub (the edge knows the sub at connect time, before the
// owner assigns a seat).
type localConn struct {
	mu        sync.Mutex
	conn      Conn
	sub       string
	spectator bool
	admitted  bool
	rejected  bool
}

// Registry tracks the sockets a replica holds for each room so it can deliver
// owner-published [Envelope]s to exactly the right local recipients. It is the
// edge side of the relay: it contains no game logic, only selector matching.
//
// The cluster edge registers locally held sockets for remotely owned rooms.
// When an outbound envelope arrives on the room's channel, Deliver routes it by
// [Target] to matching local recipients.
type Registry struct {
	mu    sync.RWMutex
	rooms map[string][]*localConn
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	return &Registry{rooms: map[string][]*localConn{}}
}

// AddPlayer registers an edge-held player connection, initially pending owner
// admission. Multiple sockets for the same (room, sub) are allowed.
func (r *Registry) AddPlayer(roomID, sub string, conn Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[roomID] = append(r.rooms[roomID], &localConn{conn: conn, sub: sub})
}

// AddSpectator registers a spectator socket for a room. spectatorID is an
// opaque per-connection key used for admission and removal. Pending spectators
// receive targeted replies only, never room broadcasts.
func (r *Registry) AddSpectator(roomID, spectatorID string, conn Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[roomID] = append(r.rooms[roomID], &localConn{conn: conn, sub: spectatorID, spectator: true})
}

// RemovePlayer drops one player socket for (room, sub). When several sockets
// share the same sub (multi-tab), only the first matching entry is removed so
// the remaining tabs keep receiving frames.
func (r *Registry) RemovePlayer(roomID, sub string) {
	r.removeOne(roomID, sub, false)
}

// RemoveSpectator drops a spectator's socket from a room.
func (r *Registry) RemoveSpectator(roomID, spectatorID string) {
	r.removeOne(roomID, spectatorID, true)
}

func (r *Registry) removeOne(roomID, key string, spectator bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := r.rooms[roomID]
	kept := entries[:0]
	removed := false
	for _, e := range entries {
		if !removed && e.spectator == spectator && e.sub == key {
			removed = true
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
		e.mu.Lock()
		if !e.rejected && matches(env.Target, e) {
			if e.spectator && env.Target.Kind == TargetSpectator {
				if fatal, _ := env.Payload["fatal"].(bool); fatal {
					e.rejected = true
				} else if kind, _ := env.Payload["type"].(string); kind == "spectator_state" || kind == "game_over" {
					// Only the owner's connection-specific initial snapshot admits
					// a viewer. A broadcast (including game_over) never does.
					e.admitted = true
				}
			}
			e.conn.Send(env.Payload)
		}
		e.mu.Unlock()
	}
}

// matches decides whether a registered socket should receive an envelope.
// Sub/All target only players; Spectators targets every spectator and Spectator
// targets a single spectator by id. This is the single point that enforces "a
// player only receives their own seat view" across replicas.
func matches(t Target, e *localConn) bool {
	switch t.Kind {
	case TargetSpectators:
		return e.spectator && e.admitted
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
