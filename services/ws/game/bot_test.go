package game

import (
	"reflect"
	"testing"
)

func TestPickMoveReturnsFirstValidCard(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	hand := []Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Spades, Rank: Five},
		{Suit: Spades, Rank: Nine},
	}

	move, ok := PickMove(state, hand)

	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != (Card{Suit: Spades, Rank: Five}) {
		t.Fatalf("picked %+v, want first valid card", move.Card)
	}
	if move.FaceDown {
		t.Fatal("expected playable card, got face-down move")
	}
}

func TestPickMoveReturnsFirstCardFaceDownWhenNoValidMovesExist(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	hand := []Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Clubs, Rank: Three},
	}

	move, ok := PickMove(state, hand)

	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != hand[0] || !move.FaceDown {
		t.Fatalf("picked %+v faceDown=%t, want first card face-down", move.Card, move.FaceDown)
	}
}

func TestPickMoveIsDeterministic(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	hand := []Card{
		{Suit: Hearts, Rank: Seven},
		{Suit: Spades, Rank: Five},
		{Suit: Spades, Rank: Nine},
	}

	first, firstOK := PickMove(state, hand)
	second, secondOK := PickMove(state, hand)

	if firstOK != secondOK || !reflect.DeepEqual(first, second) {
		t.Fatalf("moves differ: first=%+v ok=%t second=%+v ok=%t", first, firstOK, second, secondOK)
	}
}
