package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return New(client, time.Hour), mr
}

func sampleState() game.GameState {
	state := game.NewGameState()
	state.Hands[0] = []game.Card{{Suit: game.Spades, Rank: game.Seven}, {Suit: game.Hearts, Rank: game.Ace}}
	state.Hands[1] = []game.Card{{Suit: game.Clubs, Rank: game.King}}
	state.Hands[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Two}}
	state.Hands[3] = []game.Card{{Suit: game.Hearts, Rank: game.Six}}
	state.FaceDown[0] = []game.Card{{Suit: game.Clubs, Rank: game.Three}}
	state.FaceDown[2] = []game.Card{{Suit: game.Diamonds, Rank: game.Jack}}
	state.Board[game.Spades] = game.SuitSequence{Low: game.Six, High: game.Eight}
	state.Closed[game.Hearts] = true
	state.CloseMethod = game.CloseLow
	state.CurrentPlayer = 2
	return state
}

func sampleSnapshot() RoomSnapshot {
	return RoomSnapshot{
		State: sampleState(),
		Players: []PersistedPlayer{
			{Sub: "alice-id", DisplayName: "Alice", Avatar: "https://cdn/alice.png", IsGuest: true, Ready: true, Index: 0},
			{Sub: "bob-id", DisplayName: "Bob", Ready: true, Index: 1},
			{DisplayName: "Bot 1", IsBot: true, Ready: true, Index: 2},
			{DisplayName: "Bot 2", IsBot: true, Ready: true, Index: 3},
		},
		Phase:            1,
		Started:          true,
		StartedAt:        time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		TurnExpiresAt:    time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC),
		TurnTimerSeconds: 30,
		TurnTimerToken:   7,
		RematchVotes:     []int{0, 2},
	}
}

func TestSaveLoadRoundTripsFullSnapshot(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	want := sampleSnapshot()

	if err := store.SaveRoom(ctx, "room-1", want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.LoadRoom(ctx, "room-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Game state.
	for player := 0; player < game.PlayerCount; player++ {
		if !equalCards(got.State.Hands[player], want.State.Hands[player]) {
			t.Fatalf("hand %d mismatch: got %+v want %+v", player, got.State.Hands[player], want.State.Hands[player])
		}
		if !equalCards(got.State.FaceDown[player], want.State.FaceDown[player]) {
			t.Fatalf("face-down %d mismatch: got %+v want %+v", player, got.State.FaceDown[player], want.State.FaceDown[player])
		}
	}
	if got.State.CurrentPlayer != want.State.CurrentPlayer {
		t.Fatalf("current player: got %d want %d", got.State.CurrentPlayer, want.State.CurrentPlayer)
	}
	if got.State.CloseMethod != want.State.CloseMethod {
		t.Fatalf("close method: got %q want %q", got.State.CloseMethod, want.State.CloseMethod)
	}
	if !got.State.Closed[game.Hearts] {
		t.Fatalf("expected hearts closed in loaded state")
	}

	// Room metadata.
	if len(got.Players) != len(want.Players) {
		t.Fatalf("players: got %d want %d", len(got.Players), len(want.Players))
	}
	for i, p := range want.Players {
		if got.Players[i] != p {
			t.Fatalf("player %d mismatch: got %+v want %+v", i, got.Players[i], p)
		}
	}
	if got.Phase != want.Phase || got.Started != want.Started || got.TurnTimerSeconds != want.TurnTimerSeconds || got.TurnTimerToken != want.TurnTimerToken {
		t.Fatalf("metadata mismatch: got phase=%d started=%v timer=%d token=%d", got.Phase, got.Started, got.TurnTimerSeconds, got.TurnTimerToken)
	}
	if !got.StartedAt.Equal(want.StartedAt) || !got.TurnExpiresAt.Equal(want.TurnExpiresAt) {
		t.Fatalf("timestamps mismatch: got startedAt=%v turnExpiresAt=%v", got.StartedAt, got.TurnExpiresAt)
	}
	if len(got.RematchVotes) != len(want.RematchVotes) {
		t.Fatalf("rematch votes: got %+v want %+v", got.RematchVotes, want.RematchVotes)
	}
	for i := range want.RematchVotes {
		if got.RematchVotes[i] != want.RematchVotes[i] {
			t.Fatalf("rematch vote %d: got %d want %d", i, got.RematchVotes[i], want.RematchVotes[i])
		}
	}
}

func TestLoadReturnsNotFoundWhenKeyMissing(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.LoadRoom(ctx, "missing-room")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRemovesStateAndSubsequentLoadIsNotFound(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.SaveRoom(ctx, "room-2", sampleSnapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Delete(ctx, "room-2"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if mr.Exists(StateKey("room-2")) {
		t.Fatalf("expected key to be removed from redis")
	}

	if _, err := store.LoadRoom(ctx, "room-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSaveSetsTTLOnEveryWrite(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := New(client, 30*time.Minute)
	ctx := context.Background()

	if err := store.SaveRoom(ctx, "room-3", sampleSnapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	initialTTL := mr.TTL(StateKey("room-3"))
	if initialTTL <= 0 {
		t.Fatalf("expected positive TTL after save, got %v", initialTTL)
	}

	// Advance the simulated clock so the TTL would shrink without a refresh.
	mr.FastForward(10 * time.Minute)

	beforeRefresh := mr.TTL(StateKey("room-3"))
	if beforeRefresh >= initialTTL {
		t.Fatalf("expected TTL to shrink after fast-forward, before=%v initial=%v", beforeRefresh, initialTTL)
	}

	if err := store.SaveRoom(ctx, "room-3", sampleSnapshot()); err != nil {
		t.Fatalf("save (refresh): %v", err)
	}

	refreshedTTL := mr.TTL(StateKey("room-3"))
	if refreshedTTL <= beforeRefresh {
		t.Fatalf("expected TTL to be refreshed by save, got %v (was %v)", refreshedTTL, beforeRefresh)
	}
}

func TestStateKeyUsesRoomScopedNamespace(t *testing.T) {
	if got, want := StateKey("abc"), "room:abc:state"; got != want {
		t.Fatalf("StateKey: got %q want %q", got, want)
	}
}

func equalCards(a, b []game.Card) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
