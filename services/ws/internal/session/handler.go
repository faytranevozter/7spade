package session

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type AccessChecker interface{ CheckAccess(string) error }

// Serve connects an admitted socket to its room. It blocks for the lifetime of
// the session. The room runtime and cluster edge both use this same entry point.
type Serve func(roomID string, claims *Claims, conn *Connection, token string, spectator bool)

func Handler(secret string, access AccessChecker, serve Serve) http.HandlerFunc {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("room_id")
		if roomID == "" {
			http.Error(w, "room_id is required", http.StatusBadRequest)
			return
		}
		token := r.URL.Query().Get("token")
		claims, err := ParseToken(token, secret)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !claims.IsGuest && access != nil {
			if err := access.CheckAccess(claims.Sub); err != nil {
				http.Error(w, "account is suspended", http.StatusForbidden)
				return
			}
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serve(roomID, claims, NewConnection(conn), token, r.URL.Query().Get("role") == "spectator")
	}
}
