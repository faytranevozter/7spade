package transport

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func ConfigureLiveness(conn Conn, pongWait time.Duration) {
	conn.SetReadLimit(websocketReadLimit)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
}

// writeWebSocketJSONLocked writes one frame on a conn whose write mutex the
// caller already holds. It sets the write deadline but does not close the conn
// on error; the caller decides when to tear the connection down.
func WriteJSONLocked(conn Conn, payload map[string]any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(websocketWriteWait))
	return conn.WriteJSON(payload)
}

func WriteJSON(conn Conn, mu *sync.Mutex, payload map[string]any) error {
	mu.Lock()
	err := WriteJSONLocked(conn, payload)
	mu.Unlock()
	if err != nil {
		_ = conn.Close()
	}
	return err
}

func StartHeartbeat(conn Conn, mu *sync.Mutex, pingEvery, pongWait time.Duration) func() {
	ConfigureLiveness(conn, pongWait)
	done := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		ticker := time.NewTicker(pingEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(websocketWriteWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				mu.Unlock()
				if err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() { stopOnce.Do(func() { close(done) }) }
}
