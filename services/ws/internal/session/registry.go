package session

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ID identifies one local connection without exposing its transport to callers.
type ID string

// Delivery is the local connection seam consumed by the room runtime.
type Delivery interface {
	Send(ID, map[string]any) error
	Close(ID)
	Remove(ID)
}

// Registry owns local connection references. A room may retain an ID but never
// a Connection, so reconnect replacement and delivery remain transport-local.
type Registry struct {
	next atomic.Uint64
	mu   sync.RWMutex
	byID map[ID]*Connection
}

func NewRegistry() *Registry { return &Registry{byID: map[ID]*Connection{}} }

func (r *Registry) Add(conn *Connection) ID {
	id := ID(fmt.Sprintf("session-%d", r.next.Add(1)))
	r.mu.Lock()
	r.byID[id] = conn
	r.mu.Unlock()
	return id
}

func (r *Registry) Remove(id ID) {
	r.mu.Lock()
	delete(r.byID, id)
	r.mu.Unlock()
}

func (r *Registry) Send(id ID, payload map[string]any) error {
	r.mu.RLock()
	conn := r.byID[id]
	r.mu.RUnlock()
	if conn == nil {
		return nil
	}
	return conn.Send(payload)
}

func (r *Registry) Close(id ID) {
	r.mu.RLock()
	conn := r.byID[id]
	r.mu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}
}
