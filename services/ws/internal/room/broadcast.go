package room

// deliverToPlayers sends the same payload to an explicit set of players. Each
// is routed via deliverToSeat, so local delivery uses session.Runtime and a
// remote edge-held player is reached by a sub-targeted publication. Using per-seat
// targeting (rather than a single TargetAll) keeps inclusion/exclusion correct
// for callers that broadcast to a subset (e.g. excluding the reconnecting
// player); a room has at most game.PlayerCount seats so the publish count is
// bounded.
func (room *room) deliverToPlayers(players []*player, payload map[string]any) {
	for _, p := range players {
		room.deliverToSeat(p, payload)
	}
}

func (room *room) broadcastPlayerConnection(messageType string, displayName string, playerIndex int) {
	room.mu.Lock()
	message := map[string]any{"type": messageType, "display_name": displayName, "player_index": playerIndex}
	players := make([]*player, 0, len(room.players))
	for _, player := range room.players {
		if player.index != playerIndex && !player.disconnected && !player.isBot {
			players = append(players, player)
		}
	}
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()

	room.deliverToPlayers(players, message)
	room.deliverToSpectators(spectators, message)
}

func (room *room) broadcastGameOver() {
	room.mu.Lock()
	room.rematchVotes = map[int]bool{}
	players := connectedPlayersLocked(room.players)
	spectators := append([]*spectator(nil), room.spectators...)
	spectatorMessage := room.gameOverMessageLocked("")
	playerMessages := make(map[*player]map[string]any, len(players))
	for _, player := range players {
		playerMessages[player] = room.gameOverMessageLocked(player.sub)
	}
	room.mu.Unlock()
	for player, message := range playerMessages {
		room.deliverToSeat(player, message)
	}
	room.deliverToSpectators(spectators, spectatorMessage)
}
