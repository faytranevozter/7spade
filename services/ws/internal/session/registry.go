package session

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ID identifies one local connection without exposing its transport to callers.
type ID string

// Delivery is the local connection seam consumed by the room runtime.
type Delivery interface {
	Send(ID, map[string]any) error
	Close(ID)
	Remove(ID)
}

// Runtime owns the lifetime of a local session. Callers provide room callbacks
// but never receive a connection object.
type Runtime interface {
	Delivery
	Run(ID, Loop)
}

// Connections is used by cluster edge forwarding, which is the only non-session
// module allowed to resolve an ID back to a concrete connection.
type Connections interface {
	Connection(ID) *Connection
}

type Loop struct {
	PingEvery time.Duration
	PongWait  time.Duration
	Access    AccessChecker
	UserID    string
	Guest     bool
	Message   func([]byte)
	Closed    func()
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

func (r *Registry) Connection(id ID) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byID[id]
}

func (r *Registry) Run(id ID, loop Loop) {
	conn := r.Connection(id)
	if conn == nil {
		if loop.Closed != nil {
			loop.Closed()
		}
		return
	}
	stopHeartbeat := conn.StartHeartbeat(loop.PingEvery, loop.PongWait)
	stopAccessCheck := StartAccessCheck(loop.Access, loop.UserID, loop.Guest, conn)
	defer func() {
		stopAccessCheck()
		stopHeartbeat()
		if loop.Closed != nil {
			loop.Closed()
		}
		_ = conn.Close()
	}()
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if loop.Message != nil {
			loop.Message(payload)
		}
	}
}
