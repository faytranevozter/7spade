package session

import (
	"errors"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/protocol"
)

const accessCheckEvery = 5 * time.Second

// StartAccessCheck closes a registered player's socket soon after the API revokes
// access, including when the player is idle between client messages.
func StartAccessCheck(checker AccessChecker, userID string, isGuest bool, conn *Connection) func() {
	if checker == nil || isGuest {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(accessCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if errors.Is(checker.CheckAccess(userID), protocol.ErrAccessDenied) {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}
