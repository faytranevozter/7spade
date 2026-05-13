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

func TestSaveLoadRoundTripsFullGameState(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	want := sampleState()

	if err := store.Save(ctx, "room-1", want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Load(ctx, "room-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	for player := 0; player < game.PlayerCount; player++ {
		if !equalCards(got.Hands[player], want.Hands[player]) {
			t.Fatalf("hand %d mismatch: got %+v want %+v", player, got.Hands[player], want.Hands[player])
		}
		if !equalCards(got.FaceDown[player], want.FaceDown[player]) {
			t.Fatalf("face-down %d mismatch: got %+v want %+v", player, got.FaceDown[player], want.FaceDown[player])
		}
	}
	if got.CurrentPlayer != want.CurrentPlayer {
		t.Fatalf("current player: got %d want %d", got.CurrentPlayer, want.CurrentPlayer)
	}
	if got.CloseMethod != want.CloseMethod {
		t.Fatalf("close method: got %q want %q", got.CloseMethod, want.CloseMethod)
	}
	if seq, ok := got.Board[game.Spades]; !ok || seq != want.Board[game.Spades] {
		t.Fatalf("board[spades]: got %+v ok=%v want %+v", seq, ok, want.Board[game.Spades])
	}
	if !got.Closed[game.Hearts] {
		t.Fatalf("expected hearts closed in loaded state")
	}
	if got.Closed[game.Spades] {
		t.Fatalf("did not expect spades closed in loaded state")
	}
}

func TestLoadReturnsNotFoundWhenKeyMissing(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.Load(ctx, "missing-room")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRemovesStateAndSubsequentLoadIsNotFound(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "room-2", sampleState()); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Delete(ctx, "room-2"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if mr.Exists(StateKey("room-2")) {
		t.Fatalf("expected key to be removed from redis")
	}

	if _, err := store.Load(ctx, "room-2"); !errors.Is(err, ErrNotFound) {
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

	if err := store.Save(ctx, "room-3", sampleState()); err != nil {
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

	if err := store.Save(ctx, "room-3", sampleState()); err != nil {
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
