package room

import (
	"sync"

	"github.com/faytranevozter/7spade/services/ws/game"
	"github.com/faytranevozter/7spade/services/ws/store"
)

type memoryStateStore struct {
	mu        sync.Mutex
	snapshots map[string]roomSnapshot
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{snapshots: map[string]roomSnapshot{}}
}

func (store *memoryStateStore) SaveRoom(roomID string, snap roomSnapshot) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.snapshots[roomID] = cloneRoomSnapshot(snap)
}

func (store *memoryStateStore) LoadRoom(roomID string) (roomSnapshot, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	snap, ok := store.snapshots[roomID]
	if !ok {
		return roomSnapshot{}, false
	}
	return cloneRoomSnapshot(snap), true
}

func (store *memoryStateStore) DeleteRoom(roomID string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.snapshots, roomID)
}

// cloneRoomSnapshot deep-copies a snapshot so the in-memory store doesn't alias
// the caller's live state (mirrors the JSON copy the Redis adapter performs).
func cloneRoomSnapshot(snap roomSnapshot) roomSnapshot {
	clone := snap
	clone.state = cloneGameState(snap.state)
	clone.players = append([]persistedPlayer(nil), snap.players...)
	clone.rematchVotes = append([]int(nil), snap.rematchVotes...)
	return clone
}

func toStoreSnapshot(snap roomSnapshot) store.RoomSnapshot {
	players := make([]store.PersistedPlayer, 0, len(snap.players))
	for _, p := range snap.players {
		players = append(players, store.PersistedPlayer{
			Sub:         p.sub,
			DisplayName: p.displayName,
			Avatar:      p.avatar,
			IsGuest:     p.isGuest,
			IsBot:       p.isBot,
			Ready:       p.ready,
			Index:       p.index,
			Team:        p.team,
		})
	}
	initialHands := make([][]game.Card, len(snap.initialHands))
	for i := range snap.initialHands {
		if len(snap.initialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), snap.initialHands[i]...)
		}
	}
	var moves []store.PersistedMove
	if len(snap.moves) > 0 {
		moves = make([]store.PersistedMove, len(snap.moves))
		for i, m := range snap.moves {
			moves[i] = store.PersistedMove{
				PlayerIndex:  m.PlayerIndex,
				Suit:         string(m.Suit),
				Rank:         int(m.Rank),
				Type:         m.Type,
				AceDirection: string(m.AceDirection),
			}
		}
	}
	out := store.RoomSnapshot{
		State:            snap.state,
		Players:          players,
		Phase:            int(snap.phase),
		Started:          snap.started,
		StartedAt:        snap.startedAt,
		TurnExpiresAt:    snap.turnExpiresAt,
		TurnTimerSeconds: snap.turnTimerSeconds,
		BotDifficulty:    string(snap.botDifficulty),
		PracticeMode:     snap.practiceMode,
		TurnTimerToken:   snap.turnTimerToken,
		RematchVotes:     append([]int(nil), snap.rematchVotes...),
		InitialHands:     initialHands,
		Moves:            moves,
		Version:          snap.version,
		SavedAt:          snap.savedAt,
		SavedGameID:      snap.savedGameID,
	}
	if len(snap.gameDeltas) > 0 {
		deltas := make(map[string]store.PlayerDelta, len(snap.gameDeltas))
		for k, v := range snap.gameDeltas {
			deltas[k] = store.PlayerDelta(v)
		}
		out.Deltas = deltas
	}
	return out
}

func fromStoreSnapshot(snap store.RoomSnapshot) roomSnapshot {
	players := make([]persistedPlayer, 0, len(snap.Players))
	for _, p := range snap.Players {
		players = append(players, persistedPlayer{
			sub:         p.Sub,
			displayName: p.DisplayName,
			avatar:      p.Avatar,
			isGuest:     p.IsGuest,
			isBot:       p.IsBot,
			ready:       p.Ready,
			index:       p.Index,
			team:        p.Team,
		})
	}
	initialHands := make([][]game.Card, len(snap.InitialHands))
	for i := range snap.InitialHands {
		if len(snap.InitialHands[i]) > 0 {
			initialHands[i] = append([]game.Card(nil), snap.InitialHands[i]...)
		}
	}
	var moves []recordedMove
	if len(snap.Moves) > 0 {
		moves = make([]recordedMove, len(snap.Moves))
		for i, m := range snap.Moves {
			moves[i] = recordedMove{
				PlayerIndex:  m.PlayerIndex,
				Suit:         game.Suit(m.Suit),
				Rank:         game.Rank(m.Rank),
				Type:         m.Type,
				AceDirection: game.CloseMethod(m.AceDirection),
			}
		}
	}
	roomSnap := roomSnapshot{
		state:            snap.State,
		players:          players,
		phase:            roomPhase(snap.Phase),
		started:          snap.Started,
		startedAt:        snap.StartedAt,
		turnExpiresAt:    snap.TurnExpiresAt,
		turnTimerSeconds: snap.TurnTimerSeconds,
		botDifficulty:    normalizeBotDifficulty(snap.BotDifficulty),
		practiceMode:     snap.PracticeMode,
		turnTimerToken:   snap.TurnTimerToken,
		rematchVotes:     append([]int(nil), snap.RematchVotes...),
		initialHands:     initialHands,
		moves:            moves,
		version:          snap.Version,
		savedAt:          snap.SavedAt,
		savedGameID:      snap.SavedGameID,
	}
	if len(snap.Deltas) > 0 {
		deltas := make(map[string]playerDelta, len(snap.Deltas))
		for k, v := range snap.Deltas {
			deltas[k] = playerDelta(v)
		}
		roomSnap.gameDeltas = deltas
	}
	return roomSnap
}

type Snapshot = roomSnapshot

func ToStoreSnapshot(s Snapshot) store.RoomSnapshot   { return toStoreSnapshot(s) }
func FromStoreSnapshot(s store.RoomSnapshot) Snapshot { return fromStoreSnapshot(s) }
