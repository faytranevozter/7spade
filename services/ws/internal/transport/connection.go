// Package transport owns WebSocket I/O and liveness. Conn is also implemented
// by in-memory connections in runtime tests; it never carries room state.
package transport

import "time"

// Conn preserves the identity of a connection across reconnects. Implementations
// must be comparable (normally pointers). One reader is allowed; callers share
// a write mutex between heartbeat and application writes.
type Conn interface {
	ReadJSON(any) error
	ReadMessage() (int, []byte, error)
	WriteJSON(any) error
	WriteMessage(int, []byte) error
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetPongHandler(func(string) error)
	Close() error
}

const (
	websocketWriteWait = 10 * time.Second
	websocketReadLimit = 32 * 1024
)
