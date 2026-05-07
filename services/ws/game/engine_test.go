package game

import (
	"reflect"
	"testing"
)

func TestDealIsDeterministicAndFindsSevenSpadeHolder(t *testing.T) {
	state, starter := Deal(42)
	repeated, repeatedStarter := Deal(42)

	if starter < 0 || starter >= PlayerCount {
		t.Fatalf("starter out of range: %d", starter)
	}
	if starter != repeatedStarter || !reflect.DeepEqual(state.Hands, repeated.Hands) {
		t.Fatal("expected deal with same seed to be deterministic")
	}
	if state.CurrentPlayer != starter {
		t.Fatalf("expected current player %d, got %d", starter, state.CurrentPlayer)
	}

	seen := map[Card]bool{}
	for player, hand := range state.Hands {
		if len(hand) != 13 {
			t.Fatalf("player %d got %d cards, want 13", player, len(hand))
		}
		for _, card := range hand {
			if seen[card] {
				t.Fatalf("duplicate card dealt: %+v", card)
			}
			seen[card] = true
		}
	}
	if len(seen) != 52 {
		t.Fatalf("expected 52 unique cards, got %d", len(seen))
	}
	if !containsCard(state.Hands[starter], Card{Suit: Spades, Rank: Seven}) {
		t.Fatalf("starter %d does not hold seven of spades", starter)
	}
}

func TestCardPointValuesFollowRankValues(t *testing.T) {
	checks := map[Card]int{
		{Suit: Clubs, Rank: Two}:     2,
		{Suit: Hearts, Rank: Ten}:    10,
		{Suit: Diamonds, Rank: Jack}: 11,
		{Suit: Spades, Rank: Queen}:  12,
		{Suit: Clubs, Rank: King}:    13,
		{Suit: Hearts, Rank: Ace}:    14,
	}

	for card, want := range checks {
		if got := card.PointValue(); got != want {
			t.Fatalf("%+v point value = %d, want %d", card, got, want)
		}
	}
}

func TestValidMovesAtStartRequireSevenOfSpades(t *testing.T) {
	state := NewGameState()
	hand := []Card{
		{Suit: Hearts, Rank: Seven},
		{Suit: Spades, Rank: Seven},
		{Suit: Spades, Rank: Six},
	}

	moves := ValidMoves(state, hand)

	if moves.FaceDownOnly {
		t.Fatal("expected a playable card")
	}
	assertCardsEqual(t, moves.Cards, []Card{{Suit: Spades, Rank: Seven}})
}

func TestValidMovesAllowSequenceExtensionsAndNewSevenStarts(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	state.Board[Hearts] = SuitSequence{Low: Seven, High: Seven}
	hand := []Card{
		{Suit: Spades, Rank: Five},
		{Suit: Spades, Rank: Nine},
		{Suit: Hearts, Rank: Six},
		{Suit: Diamonds, Rank: Seven},
		{Suit: Clubs, Rank: Five},
	}

	moves := ValidMoves(state, hand)

	if moves.FaceDownOnly {
		t.Fatal("expected playable cards")
	}
	assertCardsEqual(t, moves.Cards, []Card{
		{Suit: Spades, Rank: Five},
		{Suit: Spades, Rank: Nine},
		{Suit: Hearts, Rank: Six},
		{Suit: Diamonds, Rank: Seven},
	})
}

func TestValidMovesReportsFaceDownOnlyWhenNoCardIsPlayable(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	hand := []Card{
		{Suit: Spades, Rank: Four},
		{Suit: Hearts, Rank: Nine},
		{Suit: Clubs, Rank: Three},
	}

	moves := ValidMoves(state, hand)

	if !moves.FaceDownOnly {
		t.Fatalf("expected face-down only, got %+v", moves)
	}
	if len(moves.Cards) != 0 {
		t.Fatalf("expected no playable cards, got %+v", moves.Cards)
	}
}

func TestApplyMoveUpdatesBoardAndRejectsIllegalMoves(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Spades, Rank: Seven}, {Suit: Hearts, Rank: Seven}}

	updated, err := ApplyMove(state, 0, Card{Suit: Spades, Rank: Seven}, false)
	if err != nil {
		t.Fatalf("apply legal move: %v", err)
	}
	if updated.Board[Spades] != (SuitSequence{Low: Seven, High: Seven}) {
		t.Fatalf("unexpected board: %+v", updated.Board[Spades])
	}
	if containsCard(updated.Hands[0], Card{Suit: Spades, Rank: Seven}) {
		t.Fatal("played card was not removed from hand")
	}

	if _, err := ApplyMove(updated, 0, Card{Suit: Hearts, Rank: Seven}, true); err == nil {
		t.Fatal("expected face-down move to be rejected when a valid card is available")
	}
	if _, err := ApplyMove(updated, 0, Card{Suit: Clubs, Rank: Nine}, false); err == nil {
		t.Fatal("expected card not in hand to be rejected")
	}
}

func TestApplyMoveAllowsFaceDownOnlyWhenNoValidMoveExistsAndScoresPenalties(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	state.Hands[2] = []Card{{Suit: Hearts, Rank: Ten}, {Suit: Clubs, Rank: Three}}

	updated, err := ApplyMove(state, 2, Card{Suit: Hearts, Rank: Ten}, true)
	if err != nil {
		t.Fatalf("apply face-down move: %v", err)
	}
	if len(updated.FaceDown[2]) != 1 || updated.FaceDown[2][0] != (Card{Suit: Hearts, Rank: Ten}) {
		t.Fatalf("unexpected face-down cards: %+v", updated.FaceDown[2])
	}
	if containsCard(updated.Hands[2], Card{Suit: Hearts, Rank: Ten}) {
		t.Fatal("face-down card was not removed from hand")
	}

	scores := CalculateScores(updated)
	if scores[2] != 10 {
		t.Fatalf("expected player 2 score 10, got %d", scores[2])
	}
}

func TestFullGameSimulationReachesGameOver(t *testing.T) {
	state, _ := Deal(7)
	turns := 0

	for !IsGameOver(state) {
		if turns > 300 {
			t.Fatal("simulation did not finish")
		}

		player := state.CurrentPlayer
		moves := ValidMoves(state, state.Hands[player])
		card := state.Hands[player][0]
		faceDown := true
		if !moves.FaceDownOnly {
			card = moves.Cards[0]
			faceDown = false
		}

		var err error
		state, err = ApplyMove(state, player, card, faceDown)
		if err != nil {
			t.Fatalf("turn %d player %d applying %+v faceDown=%t: %v", turns, player, card, faceDown, err)
		}
		turns++
	}

	if !IsGameOver(state) {
		t.Fatal("expected game over")
	}
	for player, hand := range state.Hands {
		if len(hand) != 0 {
			t.Fatalf("player %d still has cards: %+v", player, hand)
		}
	}
}

func TestIsGameOverRequiresAllHandsToBeEmpty(t *testing.T) {
	state := NewGameState()
	if !IsGameOver(state) {
		t.Fatal("expected empty hands to be game over")
	}

	state.Hands[3] = []Card{{Suit: Diamonds, Rank: Two}}
	if IsGameOver(state) {
		t.Fatal("expected game to continue while any player has cards")
	}
}

func TestApplyAceCloseLowLocksGlobalMethod(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Spades, Rank: Ace}}
	state.Board[Spades] = SuitSequence{Low: Two, High: King}

	updated, err := ApplyAceClose(state, 0, Spades, CloseLow)
	if err != nil {
		t.Fatalf("first ace close should succeed: %v", err)
	}
	if !updated.Closed[Spades] {
		t.Fatal("expected spades to be closed")
	}
	if updated.CloseMethod != CloseLow {
		t.Fatalf("expected close method %s, got %s", CloseLow, updated.CloseMethod)
	}
	if containsCard(updated.Hands[0], Card{Suit: Spades, Rank: Ace}) {
		t.Fatal("ace was not removed from hand")
	}
}

func TestApplyAceCloseHighLocksGlobalMethod(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Hearts, Rank: Ace}}
	state.Board[Hearts] = SuitSequence{Low: Seven, High: King}

	updated, err := ApplyAceClose(state, 0, Hearts, CloseHigh)
	if err != nil {
		t.Fatalf("first ace close should succeed: %v", err)
	}
	if !updated.Closed[Hearts] {
		t.Fatal("expected hearts to be closed")
	}
	if updated.CloseMethod != CloseHigh {
		t.Fatalf("expected close method %s, got %s", CloseHigh, updated.CloseMethod)
	}
}

func TestApplyAceCloseSecondSuitSameMethodSucceeds(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.CloseMethod = CloseLow
	state.Hands[0] = []Card{{Suit: Clubs, Rank: Ace}}
	state.Board[Spades] = SuitSequence{Low: Two, High: King}
	state.Board[Clubs] = SuitSequence{Low: Two, High: King}
	state.Closed = map[Suit]bool{Spades: true}

	updated, err := ApplyAceClose(state, 0, Clubs, CloseLow)
	if err != nil {
		t.Fatalf("second ace close with same method should succeed: %v", err)
	}
	if !updated.Closed[Clubs] {
		t.Fatal("expected clubs to be closed")
	}
}

func TestApplyAceCloseSecondSuitOppositeMethodRejected(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.CloseMethod = CloseLow
	state.Hands[0] = []Card{{Suit: Hearts, Rank: Ace}}
	state.Board[Spades] = SuitSequence{Low: Two, High: King}
	state.Board[Hearts] = SuitSequence{Low: Seven, High: King}
	state.Closed = map[Suit]bool{Spades: true}

	_, err := ApplyAceClose(state, 0, Hearts, CloseHigh)
	if err == nil {
		t.Fatal("expected opposite close method to be rejected")
	}
}

func TestApplyAceCloseRejectsUnstartedSuit(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Diamonds, Rank: Ace}}

	_, err := ApplyAceClose(state, 0, Diamonds, CloseLow)
	if err == nil {
		t.Fatal("expected ace close on unstarted suit to be rejected")
	}
}

func TestApplyAceCloseRejectsAlreadyClosedSuit(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.CloseMethod = CloseHigh
	state.Hands[0] = []Card{{Suit: Spades, Rank: Ace}}
	state.Board[Spades] = SuitSequence{Low: Seven, High: King}
	state.Closed = map[Suit]bool{Spades: true}

	_, err := ApplyAceClose(state, 0, Spades, CloseHigh)
	if err == nil {
		t.Fatal("expected re-close of already closed suit to be rejected")
	}
}

func TestApplyAceCloseRejectsPlayerWithoutAce(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Board[Spades] = SuitSequence{Low: Two, High: King}

	_, err := ApplyAceClose(state, 0, Spades, CloseLow)
	if err == nil {
		t.Fatal("expected rejection when player does not hold the ace")
	}
}

func TestApplyAceCloseRejectsCloseLowWithoutTwo(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Spades, Rank: Ace}}
	state.Board[Spades] = SuitSequence{Low: Three, High: Seven}

	_, err := ApplyAceClose(state, 0, Spades, CloseLow)
	if err == nil {
		t.Fatal("expected close-low to be rejected when Low != 2")
	}
}

func TestApplyAceCloseRejectsCloseHighWithoutKing(t *testing.T) {
	state := NewGameState()
	state.CurrentPlayer = 0
	state.Hands[0] = []Card{{Suit: Hearts, Rank: Ace}}
	state.Board[Hearts] = SuitSequence{Low: Seven, High: Queen}

	_, err := ApplyAceClose(state, 0, Hearts, CloseHigh)
	if err == nil {
		t.Fatal("expected close-high to be rejected when High != K")
	}
}

func TestValidMovesExcludesClosedSuits(t *testing.T) {
	state := NewGameState()
	state.CloseMethod = CloseHigh
	state.Closed = map[Suit]bool{Spades: true}
	state.Board[Spades] = SuitSequence{Low: Three, High: Queen}
	state.Board[Hearts] = SuitSequence{Low: Five, High: Nine}

	hand := []Card{
		{Suit: Spades, Rank: Two},
		{Suit: Spades, Rank: King},
		{Suit: Hearts, Rank: Four},
		{Suit: Hearts, Rank: Ten},
	}

	moves := ValidMoves(state, hand)

	assertCardsEqual(t, moves.Cards, []Card{
		{Suit: Hearts, Rank: Four},
		{Suit: Hearts, Rank: Ten},
	})
	if moves.FaceDownOnly {
		t.Fatal("expected playable cards from non-closed suit")
	}
}

func TestCalculateScoresAceValueLowClose(t *testing.T) {
	state := NewGameState()
	state.CloseMethod = CloseLow
	state.FaceDown[0] = []Card{
		{Suit: Hearts, Rank: Ace},
		{Suit: Clubs, Rank: Five},
		{Suit: Spades, Rank: Ace},
	}

	scores := CalculateScores(state)

	if scores[0] != 7 {
		t.Fatalf("expected player 0 score 7 (1+5+1 with low close), got %d", scores[0])
	}
}

func TestCalculateScoresAceValueHighClose(t *testing.T) {
	state := NewGameState()
	state.CloseMethod = CloseHigh
	state.FaceDown[0] = []Card{
		{Suit: Diamonds, Rank: Ace},
		{Suit: Spades, Rank: Three},
	}

	scores := CalculateScores(state)

	if scores[0] != 17 {
		t.Fatalf("expected player 0 score 17 (14+3 with high close), got %d", scores[0])
	}
}

func TestCalculateScoresAceDefaultsToRankWithoutCloseMethod(t *testing.T) {
	state := NewGameState()
	state.FaceDown[1] = []Card{
		{Suit: Clubs, Rank: Ace},
	}

	scores := CalculateScores(state)

	if scores[1] != 14 {
		t.Fatalf("expected player 1 score 14 (ace default rank), got %d", scores[1])
	}
}

func assertCardsEqual(t *testing.T, got, want []Card) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cards mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}
