package room

import (
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

// playBotIfNeeded schedules an auto-move when it is currently a bot's turn.
// Caller must NOT hold room.mu.
func (room *room) playBotIfNeeded() {
	// Bot auto-play is an authoritative action: only the owning replica drives
	// it (no-op for a demoted owner so a failed-over replica doesn't double-play).
	if !room.isOwnerOrSolo() {
		return
	}
	room.mu.Lock()
	if !room.started || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	idx := room.state.CurrentPlayer
	if idx < 0 || idx >= len(room.players) || !room.players[idx].isBot {
		room.mu.Unlock()
		return
	}
	room.mu.Unlock()

	time.AfterFunc(botMoveDelay, func() {
		room.executeBotMove(idx)
	})
}

func (room *room) executeBotMove(botIdx int) {
	room.mu.Lock()
	// Re-check ownership here: this runs on a delayed timer, and the replica
	// may have lost the room lease (failover/demotion) in the meantime. A
	// demoted owner must not apply or persist a bot move — the new owner owns
	// that authority and would double-play.
	if !room.isOwnerOrSolo() {
		room.mu.Unlock()
		return
	}
	if !room.started || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	if room.state.CurrentPlayer != botIdx {
		room.mu.Unlock()
		return
	}
	if botIdx < 0 || botIdx >= len(room.players) || !room.players[botIdx].isBot {
		room.mu.Unlock()
		return
	}
	move, ok := game.PickMoveWithDifficulty(room.state, botIdx, room.botDifficulty)
	if !ok {
		room.mu.Unlock()
		return
	}
	state, rec, err := applyBotMove(room.state, botIdx, move)
	if err != nil {
		log.Printf("bot move failed: %v", err)
		room.mu.Unlock()
		return
	}
	room.state = state
	room.moves = append(room.moves, rec)
	room.persistLocked()
	gameOver := game.IsGameOver(room.state)
	if !gameOver {
		room.startTurnTimerLocked()
	}
	room.mu.Unlock()

	if gameOver {
		room.saveGameResult()
		room.broadcastGameOver()
		return
	}
	room.broadcastState()
	room.playBotIfNeeded()
}
