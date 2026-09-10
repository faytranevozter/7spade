package session

import (
	"net/http"

	"github.com/faytranevozter/7spade/services/ws/internal/transport"
)

type AccessChecker interface{ CheckAccess(string) error }

// Serve is the post-authentication admission callback. It chooses local room or
// edge handling and blocks for the admitted session's lifetime.
type Serve func(roomID string, claims *Claims, sessionID ID, token string, spectator bool)

func Handler(secret string, access AccessChecker, sessions *Registry, serve Serve) http.HandlerFunc {
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
		conn, err := transport.Upgrade(w, r)
		if err != nil {
			return
		}
		connection := NewConnection(conn)
		sessionID := sessions.Add(connection)
		defer sessions.Remove(sessionID)
		serve(roomID, claims, sessionID, token, r.URL.Query().Get("role") == "spectator")
	}
}
