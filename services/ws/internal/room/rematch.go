package room

import (
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

// handleRematchVoteLocked mutates voting under the caller's lock and returns
// notifications to run after that caller releases the lock.
func (room *room) handleRematchVoteLocked(player *player) func() {
	if !game.IsGameOver(room.state) {
		return func() { player.sendError("rematch is only available after game over") }
	}
	if !room.controlEnabled(controlNewGameStarts) {
		return func() { player.sendError("new game starts are temporarily unavailable") }
	}
	if room.rematchVotes == nil {
		room.rematchVotes = map[int]bool{}
	}
	firstVote := len(room.rematchVotes) == 0
	room.rematchVotes[player.index] = true
	if firstVote {
		room.startRematchTimerLocked()
	}
	if len(room.rematchVotes) < connectedHumanPlayerCountLocked(room.players) {
		countdown := room.rematchCountdownMessageLocked()
		return func() {
			if firstVote {
				room.broadcastRematchCountdown(countdown)
			}
			room.broadcastRematchStatus()
		}
	}
	room.startRematchGameLocked()
	return func() { room.broadcastState(); room.playBotIfNeeded() }
}

// startRematchGameLocked deals a fresh game in the same room, refreshes the
// room's started timestamp, and flips the API room status back to in_progress
// so a mid-game refresh of game 2+ reconnects instead of being treated as a
// finished room. Caller holds room.mu; it does not release it.
func (room *room) startRematchGameLocked() {
	room.stopRematchTimerLocked()
	state, starter := game.DealWithConfig(time.Now().UnixNano(), room.gameConfig)
	room.state = state
	room.state.CurrentPlayer = starter
	room.initialHands = make([][]game.Card, len(room.state.Hands))
	for i := range room.state.Hands {
		room.initialHands[i] = append([]game.Card(nil), room.state.Hands[i]...)
	}
	room.moves = nil
	room.savedGameID = ""
	room.rematchVotes = map[int]bool{}
	room.startedAt = time.Now().UTC()
	room.persistLocked()
	room.startTurnTimerLocked()

	updater := room.statusUpdater
	roomID := room.id
	if updater != nil {
		go func() {
			if err := updater.UpdateRoomStatus(roomID, "in_progress"); err != nil {
				log.Printf("update room status to in_progress on rematch: %v", err)
			}
		}()
	}
}

// startRematchTimerLocked opens the rematch countdown window. A token guards
// against a stale timer firing after the window was cancelled or restarted.
// Caller holds room.mu.
func (room *room) startRematchTimerLocked() {
	if room.rematchTimer != nil {
		room.rematchTimer.Stop()
	}
	window := room.rematchWindow
	if window <= 0 {
		window = defaultRematchWindow
	}
	room.rematchExpiresAt = time.Now().Add(window).UTC()
	room.rematchTimerToken++
	token := room.rematchTimerToken
	room.rematchTimer = time.AfterFunc(window, func() {
		room.handleRematchTimeout(token)
	})
}

// stopRematchTimerLocked cancels any pending countdown and invalidates its
// token so a concurrently-firing timer becomes a no-op. Caller holds room.mu.
func (room *room) stopRematchTimerLocked() {
	if room.rematchTimer != nil {
		room.rematchTimer.Stop()
		room.rematchTimer = nil
	}
	room.rematchTimerToken++
	room.rematchExpiresAt = time.Time{}
}

// handleRematchTimeout fires when the countdown expires without a unanimous
// vote. Voters drop back to the waiting room (same room, lobby phase) and the
// non-voters are removed. If nobody voted the room is torn down.
func (room *room) handleRematchTimeout(token int) {
	room.mu.Lock()
	if token != room.rematchTimerToken {
		// Superseded by a unanimous vote or a restart; ignore.
		room.mu.Unlock()
		return
	}
	room.rematchTimer = nil
	if !game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}

	// Partition connected humans into voters and non-voters. Bots are dropped
	// regardless: a fresh waiting room re-fills bot seats only when the host
	// starts the next game.
	voters := make([]*player, 0, len(room.players))
	nonVoters := make([]*player, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		if p.disconnected {
			// A held-but-disconnected seat is treated as a non-voter, but it has
			// no live socket to notify; just let its grace timer/route handle it.
			continue
		}
		if room.rematchVotes[p.index] {
			voters = append(voters, p)
		} else {
			nonVoters = append(nonVoters, p)
		}
	}

	if len(voters) == 0 {
		// Nobody wants a rematch: tear the room down and send everyone home.
		players := connectedPlayersLocked(room.players)
		room.players = nil
		room.rematchVotes = map[int]bool{}
		roomID := room.id
		store := room.store
		room.mu.Unlock()
		room.deliverToPlayers(players, map[string]any{"type": messageTypeRoomClosed})
		if store != nil {
			store.DeleteRoom(roomID)
		}
		return
	}

	// Rebuild the roster from the voters and reset it to a fresh lobby (see
	// returnToWaitingRoomLocked). Non-voters are removed.
	notify := room.returnToWaitingRoomLocked(voters, nonVoters)
	room.mu.Unlock()
	notify()
}

// returnToWaitingRoomLocked resets the room to a fresh pre-game lobby containing
// only `keep`, removing `remove` (their seats + DB membership rows + a
// room_closed notice). Bots and any players not in `keep` are dropped from the
// roster. Index 0 becomes the host (implicitly ready); everyone else must ready
// up again. Caller holds room.mu; the returned notification runs after unlocking.
func (room *room) returnToWaitingRoomLocked(keep, remove []*player) func() {
	room.stopRematchTimerLocked()

	remover := room.memberRemover
	roomID := room.id
	droppedSubs := make([]string, 0, len(remove))
	for _, p := range remove {
		if p.sub != "" && !p.isBot {
			droppedSubs = append(droppedSubs, p.sub)
		}
	}

	room.players = keep
	for i, p := range room.players {
		p.index = i
		p.ready = i == 0
	}
	room.phase = phaseLobby
	room.started = false
	room.state = game.GameState{}
	room.rematchVotes = map[int]bool{}
	room.rematchExpiresAt = time.Time{}
	room.persistLocked()

	updater := room.statusUpdater
	return func() {

		// Send the removed players away and drop their DB membership rows.
		for _, p := range remove {
			if !p.isBot {
				p.send(map[string]any{"type": messageTypeRoomClosed})
			}
		}
		if remover != nil {
			for _, sub := range droppedSubs {
				sub := sub
				go func() {
					if err := remover.RemoveRoomPlayer(roomID, sub); err != nil {
						log.Printf("remove player on waiting-room return: %v", err)
					}
				}()
			}
		}
		// Re-list the room as joinable again (it was 'finished' after game over).
		if updater != nil {
			go func() {
				if err := updater.UpdateRoomStatus(roomID, "waiting"); err != nil {
					log.Printf("update room status to waiting on waiting-room return: %v", err)
				}
			}()
		}
		room.broadcastLobbyState()
	}
}

// handleGoToWaitingRoomLocked is invoked when a player chooses to drop back to
// the waiting room after a human left during the results screen (a full rematch
// is no longer possible). All currently-connected humans move to the fresh
// lobby together; bots and any humans who already left are dropped. Caller holds
// room.mu; the returned notification runs after unlocking.
func (room *room) handleGoToWaitingRoomLocked(requester *player) func() {
	if !game.IsGameOver(room.state) {
		return func() { requester.sendError("only available after game over") }
	}
	keep := make([]*player, 0, len(room.players))
	remove := make([]*player, 0, len(room.players))
	for _, p := range room.players {
		if p.isBot {
			continue
		}
		if p.disconnected {
			// A human who already left: drop their seat + DB membership row.
			remove = append(remove, p)
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		// No connected humans remain to seat a lobby; nothing to do.
		return func() {}
	}
	return room.returnToWaitingRoomLocked(keep, remove)
}

func (room *room) broadcastRematchCountdown(message map[string]any) {
	room.mu.Lock()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, message)
}

func (room *room) rematchCountdownMessageLocked() map[string]any {
	expires := ""
	if !room.rematchExpiresAt.IsZero() {
		expires = room.rematchExpiresAt.Format(time.RFC3339Nano)
	}
	return map[string]any{
		"type":       messageTypeRematchCountdown,
		"expires_at": expires,
	}
}

func (room *room) broadcastRematchStatus() {
	room.mu.Lock()
	message := room.rematchStatusMessageLocked()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, message)
}

func (room *room) broadcastRematchCancelled() {
	room.mu.Lock()
	players := connectedPlayersLocked(room.players)
	room.mu.Unlock()
	room.deliverToPlayers(players, map[string]any{"type": messageTypeRematchCancelled})
}

func (room *room) rematchStatusMessageLocked() map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		// The rematch panel is a human decision list. Bots can't vote, so they're
		// hidden entirely. A human who dropped during voting is kept but flagged
		// as "left" so the others can see they're no longer deciding.
		if player.isBot {
			continue
		}
		players = append(players, map[string]any{
			"display_name": player.displayName,
			"player_index": player.index,
			"voted":        room.rematchVotes[player.index],
			"left":         player.disconnected,
		})
	}
	return map[string]any{
		"type":    messageTypeRematchStatus,
		"votes":   len(room.rematchVotes),
		"total":   humanPlayerCountLocked(room.players),
		"players": players,
	}
}
