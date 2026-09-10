package room

import (
	"errors"

	"github.com/faytranevozter/7spade/services/ws/internal/session"
)

// AdmitSpectator attaches an authenticated spectator to the local owner or a
// remote edge. Session owns the HTTP connection lifecycle around this call.
func (server *Manager) AdmitSpectator(roomID string, claims *tokenClaims, sessionID session.ID) error {
	if !server.controlEnabled(controlSpectatorAccess) {
		return errors.New("spectator access is temporarily unavailable")
	}
	// With the relay enabled, a spectator must be served as an edge unless
	// this replica owns the room, so it receives the owner's live envelopes
	// (state updates + spectator emotes) rather than a stale local snapshot.
	if server.relayEnabled() && !server.ownsOrCanServeSpectator(roomID) {
		server.edge.ServeSpectator(roomID, claims, sessionID, server.nextSpectatorID())
		return nil
	}
	server.handleSpectator(roomID, claims, sessionID)
	return nil
}

// AdmitPlayer attaches an authenticated player to the local owner or remote
// edge. Rejections are returned to session, which sends the fatal wire error and
// closes the admitted socket.
func (server *Manager) AdmitPlayer(roomID string, claims *tokenClaims, sessionID session.ID, token string) error {
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
			return nil
		}
		ownerToken = leaseToken
		ownerNewly = newly
	}

	room, player, joinResult, err := server.joinRoom(roomID, claims, sessionID, token)
	if err != nil {
		return err
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
	return nil
}
