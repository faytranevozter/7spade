package room

import "fmt"

// ownsRoomForTest reports whether this replica currently owns the room. Used by
// integration tests to assert single-ownership.
func (server *Manager) ownsRoomForTest(roomID string) bool {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil || gameRoom.relay == nil {
		return false
	}
	return gameRoom.relay.IsOwner()
}

// demoteRoomForTest simulates this replica losing ownership of a room (e.g. a
// crash or lease loss) without tearing down the process, so a failover test can
// let another replica claim the room. Test-only.
func (server *Manager) demoteRoomForTest(roomID string) {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil || gameRoom.relay == nil {
		return
	}
	gameRoom.relay.Stop()
}

// forceStartForTest marks every human ready and starts the game as the host
// would. Used by failover tests to avoid flaky websocket ready-up races.
// Test-only.
func (server *Manager) forceStartForTest(roomID string) error {
	server.mu.Lock()
	gameRoom := server.rooms[roomID]
	server.mu.Unlock()
	if gameRoom == nil {
		return fmt.Errorf("room %q not found", roomID)
	}
	gameRoom.mu.Lock()
	if gameRoom.phase != phaseLobby {
		gameRoom.mu.Unlock()
		return fmt.Errorf("room %q not in lobby", roomID)
	}
	var host *player
	for _, p := range gameRoom.players {
		if p.isBot {
			continue
		}
		p.ready = true
		p.disconnected = false
		if p.index == 0 {
			host = p
		}
	}
	if host == nil {
		gameRoom.mu.Unlock()
		return fmt.Errorf("room %q has no host", roomID)
	}
	gameRoom.mu.Unlock()
	gameRoom.handleStartGame(host)
	return nil
}
