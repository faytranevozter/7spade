package transport

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// Upgrade accepts a WebSocket without exposing the concrete socket outside
// transport.
func Upgrade(w http.ResponseWriter, r *http.Request) (Conn, error) {
	return upgrader.Upgrade(w, r, nil)
}
