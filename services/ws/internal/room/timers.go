package room

import (
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func (room *room) startTurnTimerLocked() {
	if room.turnTimer != nil {
		room.turnTimer.Stop()
	}
	room.turnExpiresAt = time.Now().Add(room.turnTimerDuration).UTC()
	room.turnTimerToken++
	token := room.turnTimerToken
	room.turnTimer = time.AfterFunc(room.turnTimerDuration, func() {
		room.handleTurnTimerExpired(token)
	})
}

func (room *room) handleTurnTimerExpired(token int) {
	// Auto-play on timeout is authoritative; only the owner fires it.
	if !room.isOwnerOrSolo() {
		return
	}
	room.mu.Lock()
	if !room.started || token != room.turnTimerToken || game.IsGameOver(room.state) {
		room.mu.Unlock()
		return
	}
	playerIndex := room.state.CurrentPlayer
	botMove, ok := game.PickMoveWithDifficulty(room.state, playerIndex, room.botDifficulty)
	if !ok {
		room.mu.Unlock()
		return
	}
	state, rec, err := applyBotMove(room.state, playerIndex, botMove)
	if err != nil {
		log.Printf("auto-play move failed: %v", err)
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
