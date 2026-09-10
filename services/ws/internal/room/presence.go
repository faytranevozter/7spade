package room

import (
	"context"
	"log"
	"time"
)

// presenceHeartbeat is shorter than the presence store TTL so a connected user
// never lapses offline between refreshes.
const presenceHeartbeat = 25 * time.Second

// startPresence marks a registered user online and starts a heartbeat that
// refreshes the TTL until the returned stop func is called. A no-op (returning
// an empty stop) for guests or when presence is disabled. The presence value is
// the user's room id only once the game is actually in progress; in the lobby
// it's reported as "online but not in a game" (empty room id) so friends aren't
// shown a "Watch" link for a game that hasn't started.
func (server *Manager) startPresence(claims *tokenClaims, room *room) func() {
	if server.presence == nil || claims.IsGuest || claims.Sub == "" {
		return func() {}
	}
	mark := func() {
		room.mu.Lock()
		roomID := ""
		if room.phase == phasePlaying {
			roomID = room.id
		}
		room.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.presence.Online(ctx, claims.Sub, roomID); err != nil {
			log.Printf("presence: mark online: %v", err)
		}
	}
	mark()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(presenceHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mark()
			}
		}
	}()
	return func() { close(done) }
}
