package room

import (
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func (room *room) broadcastState() {
	room.mu.Lock()
	type stateSnapshot struct {
		player  *player
		message map[string]any
	}
	snapshots := make([]stateSnapshot, 0, len(room.players))
	for _, player := range room.players {
		// Skip bots (never receive) and disconnected players. A connected
		// player whose session lives on another replica has an empty session ID but
		// is not disconnected; deliverToSeat reaches them through the Relay.
		if player.isBot || player.disconnected {
			continue
		}
		snapshots = append(snapshots, stateSnapshot{player: player, message: room.stateMessageFor(player.index)})
	}
	spectatorMsg := room.spectatorStateMessageLocked()
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()
	for _, snapshot := range snapshots {
		room.deliverToSeat(snapshot.player, snapshot.message)
	}
	room.deliverToSpectators(spectators, spectatorMsg)
}

// broadcastToSpectators fans a single message out to all current spectators,
// snapshotting the slice under the lock and sending off-lock (mirrors
// broadcastState's pattern). Used for game_over, emotes, and connection events.
func (room *room) broadcastToSpectators(message map[string]any) {
	room.mu.Lock()
	spectators := append([]*spectator(nil), room.spectators...)
	room.mu.Unlock()
	room.deliverToSpectators(spectators, message)
}

func (room *room) stateMessageFor(playerIndex int) map[string]any {
	moves := game.ValidMoves(room.state, room.state.Hands[playerIndex])
	validCards := map[game.Card]bool{}
	for _, card := range moves.Cards {
		validCards[card] = true
	}
	// A closable Ace is a legal play (via close), so mark it valid in the hand
	// and surface which ends are available so the client can prompt low/high.
	aceCloseOptions := make([]map[string]any, 0, len(moves.AceCloses))
	for _, option := range moves.AceCloses {
		validCards[game.Card{Suit: option.Suit, Rank: game.Ace}] = true
		aceCloseOptions = append(aceCloseOptions, map[string]any{
			"suit":     string(option.Suit),
			"can_low":  option.CanLow,
			"can_high": option.CanHigh,
		})
	}

	yourHand := make([]map[string]any, 0, len(room.state.Hands[playerIndex]))
	for _, card := range room.state.Hands[playerIndex] {
		yourHand = append(yourHand, cardPayload(card, validCards[card]))
	}

	yourFaceDown := make([]map[string]any, 0, len(room.state.FaceDown[playerIndex]))
	for _, card := range room.state.FaceDown[playerIndex] {
		yourFaceDown = append(yourFaceDown, cardPayload(card, false))
	}

	opponents := make([]map[string]any, 0, len(room.players)-1)
	for i := 1; i < len(room.players); i++ {
		idx := (playerIndex + i) % len(room.players)
		player := room.players[idx]
		opponentPayload := map[string]any{
			"user_id":        player.sub,
			"display_name":   player.displayName,
			"player_index":   player.index,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
			"disconnected":   player.disconnected,
			"team":           player.team,
		}
		if room.gameConfig.TeamMode == game.Team2v2 && player.team == room.players[playerIndex].team {
			opponentPayload["is_teammate"] = true
			teammateHand := make([]map[string]any, 0, len(room.state.Hands[player.index]))
			for _, card := range room.state.Hands[player.index] {
				teammateHand = append(teammateHand, map[string]any{
					"suit": string(card.Suit),
					"rank": rankString(card.Rank),
				})
			}
			opponentPayload["hand"] = teammateHand
		}
		opponents = append(opponents, opponentPayload)
	}

	var teamInfo map[string]any
	if room.gameConfig.TeamMode == game.Team2v2 {
		myTeam := room.players[playerIndex].team
		teamPenalty := 0
		for _, p := range room.players {
			if p.team == myTeam {
				for _, card := range room.state.FaceDown[p.index] {
					teamPenalty += game.ScoreCard(card, room.state)
				}
			}
		}
		teammates := make([]string, 0)
		for _, p := range room.players {
			if p.team == myTeam && p.index != playerIndex {
				teammates = append(teammates, p.displayName)
			}
		}
		teamInfo = map[string]any{
			"team":         myTeam,
			"team_penalty": teamPenalty,
			"teammates":    teammates,
		}
	}

	payload := map[string]any{
		"type":                messageTypeStateUpdate,
		"status":              "in_progress",
		"board":               boardPayload(room.state),
		"closed_suits":        closedSuits(room.state),
		"ace_close_method":    room.state.CloseMethod,
		"ace_close_options":   aceCloseOptions,
		"your_hand":           yourHand,
		"your_facedown":       yourFaceDown,
		"your_facedown_count": len(yourFaceDown),
		"your_index":          playerIndex,
		"opponents":           opponents,
		"current_turn":        room.players[room.state.CurrentPlayer].displayName,
		"current_turn_index":  room.state.CurrentPlayer,
		"turn_ends_at":        room.turnExpiresAt.Format(time.RFC3339),
		"turn_timer_seconds":  int(room.turnTimerDuration / time.Second),
		"bot_difficulty":      string(room.botDifficulty),
		"practice_mode":       room.practiceMode,
		"spectator_count":     len(room.spectators),
	}
	if teamInfo != nil {
		payload["team_info"] = teamInfo
	}
	return payload
}

// stateMessage locks room.mu and builds the per-player state payload, so
// callers that emit a reconnected player's board (post-join, relay edge
// join) can't race a concurrent move/timer mutation of room.state or
// room.players.
func (room *room) stateMessage(playerIndex int) map[string]any {
	room.mu.Lock()
	defer room.mu.Unlock()
	return room.stateMessageFor(playerIndex)
}

// spectatorStateMessageLocked builds the redacted live-state payload for
// spectators: the public board plus every player's public info (name, avatar,
// hand COUNT, face-down COUNT, disconnected) — but never any hand cards or
// hand-derived ace-close options. This is the core no-hidden-info-leak property.
// Caller must hold room.mu.
func (room *room) spectatorStateMessageLocked() map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		players = append(players, map[string]any{
			"user_id":        player.sub,
			"display_name":   player.displayName,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"hand_count":     len(room.state.Hands[player.index]),
			"facedown_count": len(room.state.FaceDown[player.index]),
			"disconnected":   player.disconnected,
		})
	}
	currentTurn := ""
	if len(room.players) > 0 {
		currentTurn = room.players[room.state.CurrentPlayer].displayName
	}
	return map[string]any{
		"type":               messageTypeSpectatorState,
		"status":             "in_progress",
		"board":              boardPayload(room.state),
		"closed_suits":       closedSuits(room.state),
		"ace_close_method":   room.state.CloseMethod,
		"players":            players,
		"current_turn":       currentTurn,
		"turn_ends_at":       room.turnExpiresAt.Format(time.RFC3339),
		"turn_timer_seconds": int(room.turnTimerDuration / time.Second),
		"bot_difficulty":     string(room.botDifficulty),
		"practice_mode":      room.practiceMode,
		"spectator_count":    len(room.spectators),
	}
}
