package room

import (
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

type stateStore interface {
	// SaveRoom persists a room snapshot. Implementations are fire-and-forget
	// from the caller's perspective (the Redis adapter writes asynchronously),
	// so this returns no error.
	SaveRoom(roomID string, snap roomSnapshot)
	// LoadRoom returns a persisted snapshot for the room, or ok=false when none
	// exists. Called only on the join path, never on the move hot-path.
	LoadRoom(roomID string) (roomSnapshot, bool)
	// DeleteRoom drops a room's persisted snapshot when the room is torn down.
	DeleteRoom(roomID string)
}

// persistedPlayer is the durable subset of a room player.
type persistedPlayer struct {
	sub         string
	displayName string
	avatar      string
	isGuest     bool
	isBot       bool
	ready       bool
	index       int
	team        int
}

// roomSnapshot is the complete durable state of a room, used to rebuild it
// after a WS process restart.
type roomSnapshot struct {
	state            game.GameState
	players          []persistedPlayer
	phase            roomPhase
	started          bool
	startedAt        time.Time
	turnExpiresAt    time.Time
	turnTimerSeconds int
	botDifficulty    game.BotDifficulty
	practiceMode     bool
	turnTimerToken   int
	rematchVotes     []int
	initialHands     [][]game.Card
	moves            []recordedMove
	version          int64
	savedAt          time.Time
	savedGameID      string
	gameDeltas       map[string]playerDelta
}

func cloneGameState(state game.GameState) game.GameState {
	clone := game.GameState{
		Hands:         make([][]game.Card, len(state.Hands)),
		FaceDown:      make([][]game.Card, len(state.FaceDown)),
		Board:         make(map[game.Suit]game.SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[game.Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
		Config:        state.Config,
	}
	for player := range state.Hands {
		clone.Hands[player] = append([]game.Card(nil), state.Hands[player]...)
		clone.FaceDown[player] = append([]game.Card(nil), state.FaceDown[player]...)
	}
	for suit, sequence := range state.Board {
		cloned := game.SuitSequence{Low: sequence.Low, High: sequence.High}
		if len(sequence.Stacks) > 0 {
			cloned.Stacks = make(map[game.Rank]int, len(sequence.Stacks))
			for rank, count := range sequence.Stacks {
				cloned.Stacks[rank] = count
			}
		}
		clone.Board[suit] = cloned
	}
	for suit, closed := range state.Closed {
		clone.Closed[suit] = closed
	}
	return clone
}

// snapshotLocked builds a durable snapshot of the room. Caller must hold room.mu.
func (room *room) snapshotLocked() roomSnapshot {
	players := make([]persistedPlayer, 0, len(room.players))
	for _, p := range room.players {
		players = append(players, persistedPlayer{
			sub:         p.sub,
			displayName: p.displayName,
			avatar:      p.avatar,
			isGuest:     p.isGuest,
			isBot:       p.isBot,
			ready:       p.ready,
			index:       p.index,
			team:        p.team,
		})
	}
	votes := make([]int, 0, len(room.rematchVotes))
	for idx := range room.rematchVotes {
		votes = append(votes, idx)
	}
	initialHands := make([][]game.Card, len(room.initialHands))
	for i := range room.initialHands {
		if len(room.initialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), room.initialHands[i]...)
		}
	}
	deltas := make(map[string]playerDelta, len(room.gameDeltas))
	for k, v := range room.gameDeltas {
		deltas[k] = v
	}
	return roomSnapshot{
		state:            cloneGameState(room.state),
		players:          players,
		phase:            room.phase,
		started:          room.started,
		startedAt:        room.startedAt,
		turnExpiresAt:    room.turnExpiresAt,
		turnTimerSeconds: int(room.turnTimerDuration / time.Second),
		botDifficulty:    room.botDifficulty,
		practiceMode:     room.practiceMode,
		turnTimerToken:   room.turnTimerToken,
		rematchVotes:     votes,
		initialHands:     initialHands,
		moves:            append([]recordedMove(nil), room.moves...),
		savedGameID:      room.savedGameID,
		gameDeltas:       deltas,
	}
}

// persistLocked saves the current room snapshot to the state store. Caller must
// hold room.mu. The Redis adapter writes asynchronously, so this does not block
// the move hot-path.
func (room *room) persistLocked() {
	if room.store == nil {
		return
	}
	room.snapVersion++
	room.snapshotSavedAt = time.Now().UTC()
	snap := room.snapshotLocked()
	snap.version = room.snapVersion
	snap.savedAt = room.snapshotSavedAt
	room.store.SaveRoom(room.id, snap)
}

// restoreFromSnapshotLocked rebuilds a freshly-created room from a persisted
// snapshot. Players are restored as disconnected (no live socket yet); a
// reconnecting client re-attaches via the normal join path. Called during
// joinRoom on a brand-new room that has not yet been published to
// server.rooms, so no room-level synchronisation is needed (the caller holds
// server.mu and no other goroutine can observe this room).
func (room *room) restoreFromSnapshotLocked(snap roomSnapshot) {
	room.state = cloneGameState(snap.state)
	room.phase = snap.phase
	room.started = snap.started
	room.startedAt = snap.startedAt
	room.turnExpiresAt = snap.turnExpiresAt
	room.snapshotSavedAt = snap.savedAt
	room.botDifficulty = snap.botDifficulty
	if room.botDifficulty == "" {
		room.botDifficulty = game.BotMedium
	}
	room.practiceMode = snap.practiceMode
	if snap.turnTimerSeconds > 0 {
		room.turnTimerDuration = time.Duration(snap.turnTimerSeconds) * time.Second
	}
	room.turnTimerToken = snap.turnTimerToken
	room.rematchVotes = map[int]bool{}
	for _, idx := range snap.rematchVotes {
		room.rematchVotes[idx] = true
	}
	room.initialHands = make([][]game.Card, len(snap.initialHands))
	for i := range snap.initialHands {
		if len(snap.initialHands[i]) > 0 {
			room.initialHands[i] = append([]game.Card(nil), snap.initialHands[i]...)
		} else {
			room.initialHands[i] = nil
		}
	}
	room.moves = append([]recordedMove(nil), snap.moves...)
	// Restore the finished game's API result so a client reconnecting after a
	// restart still receives game_id and rating/XP deltas in game_over.
	room.savedGameID = snap.savedGameID
	if len(snap.gameDeltas) > 0 {
		room.gameDeltas = make(map[string]playerDelta, len(snap.gameDeltas))
		for k, v := range snap.gameDeltas {
			room.gameDeltas[k] = v
		}
	}
	room.players = make([]*player, 0, len(snap.players))
	for _, p := range snap.players {
		room.players = append(room.players, &player{
			sub:          p.sub,
			displayName:  p.displayName,
			avatar:       p.avatar,
			isGuest:      p.isGuest,
			isBot:        p.isBot,
			ready:        p.ready,
			index:        p.index,
			team:         p.team,
			disconnected: !p.isBot,
		})
	}
	if room.phase == phasePlaying && room.started && !game.IsGameOver(room.state) {
		room.startTurnTimerLocked()
	}
}
