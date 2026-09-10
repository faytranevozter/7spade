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

// DurableSnapshot is the stable persistence DTO at the room seam. Its exported
// fields deliberately mirror the existing Redis JSON contract; adapters own any
// storage-specific translation beyond this type.
type DurableSnapshot struct {
	State            game.GameState         `json:"state"`
	Players          []DurablePlayer        `json:"players"`
	Phase            int                    `json:"phase"`
	Started          bool                   `json:"started"`
	StartedAt        time.Time              `json:"started_at"`
	TurnExpiresAt    time.Time              `json:"turn_expires_at"`
	TurnTimerSeconds int                    `json:"turn_timer_seconds"`
	BotDifficulty    string                 `json:"bot_difficulty"`
	PracticeMode     bool                   `json:"practice_mode"`
	TurnTimerToken   int                    `json:"turn_timer_token"`
	RematchVotes     []int                  `json:"rematch_votes"`
	InitialHands     [][]game.Card          `json:"initial_hands"`
	Moves            []DurableMove          `json:"moves"`
	Version          int64                  `json:"version"`
	SavedAt          time.Time              `json:"saved_at"`
	SavedGameID      string                 `json:"saved_game_id"`
	Deltas           map[string]playerDelta `json:"deltas"`
}

type DurablePlayer struct {
	Sub         string `json:"sub"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	IsGuest     bool   `json:"is_guest"`
	IsBot       bool   `json:"is_bot"`
	Ready       bool   `json:"ready"`
	Index       int    `json:"index"`
	Team        int    `json:"team"`
}

type DurableMove struct {
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit"`
	Rank         int    `json:"rank"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

// DurableSnapshotFromLive copies private runtime state into the storage-neutral
// DTO. It is the only room-to-persistence conversion point.
func DurableSnapshotFromLive(s Snapshot) DurableSnapshot {
	players := make([]DurablePlayer, len(s.players))
	for i, p := range s.players {
		players[i] = DurablePlayer{p.sub, p.displayName, p.avatar, p.isGuest, p.isBot, p.ready, p.index, p.team}
	}
	moves := make([]DurableMove, len(s.moves))
	for i, m := range s.moves {
		moves[i] = DurableMove{m.PlayerIndex, string(m.Suit), int(m.Rank), m.Type, string(m.AceDirection)}
	}
	return DurableSnapshot{State: cloneGameState(s.state), Players: players, Phase: int(s.phase), Started: s.started, StartedAt: s.startedAt, TurnExpiresAt: s.turnExpiresAt, TurnTimerSeconds: s.turnTimerSeconds, BotDifficulty: string(s.botDifficulty), PracticeMode: s.practiceMode, TurnTimerToken: s.turnTimerToken, RematchVotes: append([]int(nil), s.rematchVotes...), InitialHands: cloneHands(s.initialHands), Moves: moves, Version: s.version, SavedAt: s.savedAt, SavedGameID: s.savedGameID, Deltas: cloneDeltas(s.gameDeltas)}
}

// LiveSnapshotFromDurable restores private runtime representation from a stable
// DTO. Missing optional fields retain the legacy zero-value semantics.
func LiveSnapshotFromDurable(s DurableSnapshot) Snapshot {
	players := make([]persistedPlayer, len(s.Players))
	for i, p := range s.Players {
		players[i] = persistedPlayer{sub: p.Sub, displayName: p.DisplayName, avatar: p.Avatar, isGuest: p.IsGuest, isBot: p.IsBot, ready: p.Ready, index: p.Index, team: p.Team}
	}
	moves := make([]recordedMove, len(s.Moves))
	for i, m := range s.Moves {
		moves[i] = recordedMove{PlayerIndex: m.PlayerIndex, Suit: game.Suit(m.Suit), Rank: game.Rank(m.Rank), Type: m.Type, AceDirection: game.CloseMethod(m.AceDirection)}
	}
	return roomSnapshot{state: cloneGameState(s.State), players: players, phase: roomPhase(s.Phase), started: s.Started, startedAt: s.StartedAt, turnExpiresAt: s.TurnExpiresAt, turnTimerSeconds: s.TurnTimerSeconds, botDifficulty: normalizeBotDifficulty(s.BotDifficulty), practiceMode: s.PracticeMode, turnTimerToken: s.TurnTimerToken, rematchVotes: append([]int(nil), s.RematchVotes...), initialHands: cloneHands(s.InitialHands), moves: moves, version: s.Version, savedAt: s.SavedAt, savedGameID: s.SavedGameID, gameDeltas: cloneDeltas(s.Deltas)}
}

func cloneHands(hands [][]game.Card) [][]game.Card {
	out := make([][]game.Card, len(hands))
	for i := range hands {
		out[i] = append([]game.Card(nil), hands[i]...)
	}
	return out
}
func cloneDeltas(deltas map[string]playerDelta) map[string]playerDelta {
	if len(deltas) == 0 {
		return nil
	}
	out := make(map[string]playerDelta, len(deltas))
	for k, v := range deltas {
		out[k] = v
	}
	return out
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
			room:         room,
		})
	}
	if room.phase == phasePlaying && room.started && !game.IsGameOver(room.state) {
		room.startTurnTimerLocked()
	}
}
