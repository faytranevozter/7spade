package room

import (
	"log"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

// Serve attaches an authenticated session to the local owner or remote edge.
func (server *Manager) Serve(roomID string, claims *tokenClaims, sessionID session.ID, token string, spectator bool) {
	if spectator {
		if !server.controlEnabled(controlSpectatorAccess) {
			if err := server.sessions.Send(sessionID, fatalErrorMessage("spectator access is temporarily unavailable")); err != nil {
				log.Printf("write spectator access error: %v", err)
			}
			server.sessions.Close(sessionID)
			return
		}
		// With the relay enabled, a spectator must be served as an edge unless
		// this replica owns the room, so it receives the owner's live envelopes
		// (state updates + spectator emotes) rather than a stale local snapshot.
		if server.relayEnabled() && !server.ownsOrCanServeSpectator(roomID) {
			server.edge.ServeSpectator(roomID, claims, sessionID, server.nextSpectatorID())
			return
		}
		server.handleSpectator(roomID, claims, sessionID)
		return
	}

	// With the relay enabled, decide this replica's role for the room. If
	// another replica owns it, take the edge path: hold the socket locally and
	// proxy to the owner. Otherwise this replica owns the room and seats the
	// player authoritatively.
	var ownerToken int64
	var ownerNewly bool
	if server.relayEnabled() {
		owned, leaseToken, newly := server.acquireOwnership(roomID)
		if !owned {
			server.edge.ServePlayer(roomID, claims, sessionID, token)
			return
		}
		ownerToken = leaseToken
		ownerNewly = newly
	}

	room, player, joinResult, err := server.joinRoom(roomID, claims, sessionID, token)
	if err != nil {
		// A join rejection (kicked, room full, already started) is fatal for this
		// socket: flag it so the client routes the user back to the lobby with the
		// reason, instead of stranding them on an empty waiting room.
		if writeErr := server.sessions.Send(sessionID, fatalErrorMessage(err.Error())); writeErr != nil {
			log.Printf("write websocket join error: %v", writeErr)
		}
		server.sessions.Close(sessionID)
		return
	}
	if server.relayEnabled() {
		server.promoteToOwner(room, ownerToken, ownerNewly)
	}

	switch joinResult {
	case joinResultLobbyJoined:
		room.broadcastLobbyState()
	case joinResultLobbyReconnected:
		room.broadcastLobbyState()
	case joinResultGameReconnected:
		player.send(room.stateMessage(player.index))
	case joinResultGameOver:
		player.send(room.gameOverMessage())
	}
	// Mark the (registered) user online for the friends-presence feature. Guests
	// have no durable identity, so they're skipped. Presence lapses via TTL on
	// disconnect (a heartbeat refreshes it while connected), which avoids
	// flapping offline on a transient reconnect.
	stop := server.startPresence(claims, room)
	defer stop()
	room.runPlayerSession(player, sessionID)
}
