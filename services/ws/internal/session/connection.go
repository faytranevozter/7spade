package session

import (
	"sync"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/transport"
)

// Connection is one session's transport handle. Its identity is stable across
// reads and delivery, so a replaced session cannot disconnect its successor.
// Socket write serialization belongs here, independent of room/participant locks.
type Connection struct {
	socket  transport.Conn
	writeMu sync.Mutex
}

func NewConnection(socket transport.Conn) *Connection { return &Connection{socket: socket} }
func (c *Connection) Send(payload map[string]any) error {
	return transport.WriteJSON(c.socket, &c.writeMu, payload)
}
func (c *Connection) Close() error                      { return c.socket.Close() }
func (c *Connection) ReadJSON(value any) error          { return c.socket.ReadJSON(value) }
func (c *Connection) ReadMessage() (int, []byte, error) { return c.socket.ReadMessage() }
func (c *Connection) StartHeartbeat(pingEvery, pongWait time.Duration) func() {
	return transport.StartHeartbeat(c.socket, &c.writeMu, pingEvery, pongWait)
}
