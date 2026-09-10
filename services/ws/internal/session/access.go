package session

import (
	"github.com/faytranevozter/7spade/services/ws/internal/transport"
)

// StartAccessCheck closes a registered player's socket soon after the API revokes
// access, including when the player is idle between client messages.
func StartAccessCheck(checker AccessChecker, userID string, isGuest bool, conn *Connection) func() {
	return transport.StartAccessCheck(checker, userID, isGuest, conn)
}
