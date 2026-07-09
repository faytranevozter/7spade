package game

import (
	"reflect"
	"testing"
)

// stateWithHand builds a fresh state with the given board sequences and places
// the hand in seat 0.
func stateWithHand(hand []Card, board map[Suit]SuitSequence) GameState {
	state := NewGameState()
	for suit, seq := range board {
		state.Board[suit] = seq
	}
	state.Hands[0] = hand
	return state
}

func TestStrategyForReturnsExpectedStrategy(t *testing.T) {
	cases := []struct {
		difficulty BotDifficulty
		want       Strategy
	}{
		{BotEasy, EasyStrategy{}},
		{BotMedium, MediumStrategy{}},
		{BotHard, HardStrategy{}},
		{BotDifficulty("nonsense"), MediumStrategy{}},
		{BotDifficulty(""), MediumStrategy{}},
	}
	for _, tc := range cases {
		got := StrategyFor(tc.difficulty)
		if reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
			t.Errorf("StrategyFor(%q) = %T, want %T", tc.difficulty, got, tc.want)
		}
	}
}

func TestEasyStrategyPreservesOriginalBehavior(t *testing.T) {
	state := stateWithHand([]Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Spades, Rank: Five},
		{Suit: Spades, Rank: Nine},
	}, map[Suit]SuitSequence{Spades: {Low: Six, High: Eight}})

	move, ok := EasyStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != (Card{Suit: Spades, Rank: Five}) || move.FaceDown {
		t.Fatalf("easy picked %+v faceDown=%t, want first valid card", move.Card, move.FaceDown)
	}
}

func TestEasyStrategyFacesDownFirstCard(t *testing.T) {
	state := stateWithHand([]Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Clubs, Rank: Three},
	}, map[Suit]SuitSequence{Spades: {Low: Six, High: Eight}})

	move, ok := EasyStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != (Card{Suit: Hearts, Rank: Nine}) || !move.FaceDown {
		t.Fatalf("easy picked %+v faceDown=%t, want first card face-down", move.Card, move.FaceDown)
	}
}

func TestEasyStrategyClosesAceInsteadOfFacingDown(t *testing.T) {
	state := stateWithHand([]Card{
		{Suit: Spades, Rank: Ace},
		{Suit: Clubs, Rank: Three},
	}, map[Suit]SuitSequence{Spades: {Low: Two, High: King}})

	move, ok := EasyStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if !move.Close || move.Card != (Card{Suit: Spades, Rank: Ace}) {
		t.Fatalf("easy picked %+v, want ace close over face-down", move)
	}
}

func TestMediumStrategyPrefersSequenceProgress(t *testing.T) {
	// Spades 6-8 extends to a length-4 sequence with the 5; clubs is a brand-new
	// suit (length 1). Medium should extend spades.
	state := stateWithHand([]Card{
		{Suit: Clubs, Rank: Seven},
		{Suit: Spades, Rank: Five},
	}, map[Suit]SuitSequence{Spades: {Low: Six, High: Eight}})

	move, ok := MediumStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != (Card{Suit: Spades, Rank: Five}) {
		t.Fatalf("medium picked %+v, want the card extending the longer sequence", move.Card)
	}
}

func TestMediumStrategyAvoidsOpponentFriendlyAceCloseWhenSafeMoveExists(t *testing.T) {
	// Spades is fully open (2..K) so the Ace could close it, but a safe normal
	// play also exists (extend hearts). The close on a wide suit with many
	// outstanding cards is opponent-friendly, so medium should keep the normal
	// play.
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Two, High: King}
	state.Board[Hearts] = SuitSequence{Low: Six, High: Eight}
	state.Hands[0] = []Card{
		{Suit: Spades, Rank: Ace},
		{Suit: Hearts, Rank: Nine},
	}
	// Opponents hold cards so opponentBenefit is non-zero.
	state.Hands[1] = []Card{{Suit: Clubs, Rank: Two}}

	move, ok := MediumStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Close {
		t.Fatalf("medium closed the ace %+v, want the safe normal play", move)
	}
	if move.Card != (Card{Suit: Hearts, Rank: Nine}) {
		t.Fatalf("medium picked %+v, want hearts nine", move.Card)
	}
}

func TestMediumStrategyFacesDownLowestPenalty(t *testing.T) {
	state := stateWithHand([]Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Clubs, Rank: Three},
	}, map[Suit]SuitSequence{Spades: {Low: Six, High: Eight}})

	move, ok := MediumStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Card != (Card{Suit: Clubs, Rank: Three}) || !move.FaceDown {
		t.Fatalf("medium picked %+v faceDown=%t, want lowest penalty face-down", move.Card, move.FaceDown)
	}
}

func TestUnknownCardsExcludeOnlyPublicAndOwnKnownCards(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	state.Hands[0] = []Card{{Suit: Hearts, Rank: Nine}}
	state.FaceDown[0] = []Card{{Suit: Clubs, Rank: Two}}
	// Distinctive opponent cards the bot must NOT know.
	opponentCard := Card{Suit: Diamonds, Rank: King}
	state.Hands[1] = []Card{opponentCard}

	unknown := unknownCards(state, 0)
	set := make(map[Card]bool, len(unknown))
	for _, c := range unknown {
		set[c] = true
	}

	// Opponent card is hidden, so it must remain in the unknown universe.
	if !set[opponentCard] {
		t.Errorf("unknownCards omitted opponent card %+v — bot is cheating", opponentCard)
	}
	// Own hand, own face-down, and board cards are known and must be excluded.
	for _, known := range []Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Clubs, Rank: Two},
		{Suit: Spades, Rank: Six},
		{Suit: Spades, Rank: Seven},
		{Suit: Spades, Rank: Eight},
	} {
		if set[known] {
			t.Errorf("unknownCards included known card %+v", known)
		}
	}
}

func TestUnknownCardsExcludeClosedSuitAce(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Two, High: King}
	state.Closed[Spades] = true
	state.Hands[0] = []Card{{Suit: Hearts, Rank: Nine}}

	for _, c := range unknownCards(state, 0) {
		if c == (Card{Suit: Spades, Rank: Ace}) {
			t.Fatal("closed suit ace should be known, not in unknown universe")
		}
	}
}

// Multi-deck: seeing one copy of a card must not mark every deck copy known.
// The remaining copies stay in the unknown universe for opponent inference.
func TestUnknownCardsMultiDeckPreservesUnseenCopies(t *testing.T) {
	state := NewGameState()
	state.Config = GameConfig{PlayerCount: 4, DeckCount: 2, ScoringMode: ScoringRankValue, TeamMode: TeamFFA, StartingSuit: Spades}
	// One visible Nine of Hearts (bot's hand); deck holds two copies.
	nineH := Card{Suit: Hearts, Rank: Nine}
	state.Hands[0] = []Card{nineH}

	unknown := unknownCards(state, 0)
	count := 0
	for _, c := range unknown {
		if c == nineH {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("unknown copies of %+v = %d, want 1 (one known, one still unknown)", nineH, count)
	}
	// Full multi-deck universe is 104 cards; one known → 103 unknown.
	if len(unknown) != 103 {
		t.Fatalf("unknownCards len = %d, want 103", len(unknown))
	}
}

func TestHardStrategyDelaysAceCloseWhenOpponentBenefitIsHigh(t *testing.T) {
	// Spades wide open with many outstanding cards => closing benefits opponents.
	// A decent normal play exists (extend hearts), so hard should delay the close.
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Two, High: King}
	state.Board[Hearts] = SuitSequence{Low: Six, High: Eight}
	state.Hands[0] = []Card{
		{Suit: Spades, Rank: Ace},
		{Suit: Hearts, Rank: Nine},
	}
	state.Hands[1] = []Card{{Suit: Clubs, Rank: Two}, {Suit: Diamonds, Rank: Three}}

	move, ok := HardStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.Close {
		t.Fatalf("hard closed the ace %+v, want it to delay the close", move)
	}
}

func TestHardStrategyClosesAceWhenNoBetterAlternative(t *testing.T) {
	// Only legal action is the ace close (other card is unplayable and would be
	// a face-down penalty). Hard should close.
	state := stateWithHand([]Card{
		{Suit: Spades, Rank: Ace},
		{Suit: Clubs, Rank: Three},
	}, map[Suit]SuitSequence{Spades: {Low: Two, High: King}})

	move, ok := HardStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if !move.Close || move.Card != (Card{Suit: Spades, Rank: Ace}) {
		t.Fatalf("hard picked %+v, want the ace close", move)
	}
}

func TestHardStrategyFacesDownLowestExpectedRisk(t *testing.T) {
	// Clubs is closed: the club card is dead weight (safest discard) even though
	// a heart with a lower point value exists but sits in an open suit closer to
	// play. Hard should discard the dead club.
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	state.Board[Hearts] = SuitSequence{Low: Six, High: Eight}
	state.Board[Clubs] = SuitSequence{Low: Two, High: King}
	state.Closed[Clubs] = true
	state.Hands[0] = []Card{
		{Suit: Hearts, Rank: Four},
		{Suit: Clubs, Rank: Five},
	}

	move, ok := HardStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if !move.FaceDown {
		t.Fatalf("expected a face-down move, got %+v", move)
	}
	if move.Card != (Card{Suit: Clubs, Rank: Five}) {
		t.Fatalf("hard discarded %+v, want the dead closed-suit card", move.Card)
	}
}

func TestHardStrategyDefensiveBlock(t *testing.T) {
	// Two playable cards extend different suits to the same sequence length.
	// Hearts has many outstanding cards (opponents likely benefit / it's worth
	// pushing toward a bound), diamonds has few. The defensive heuristic should
	// favour pushing the contested suit. We assert hard picks a deterministic,
	// progress-equal card driven by the defensive/opponent terms rather than
	// crashing, and that the move is a normal play.
	state := NewGameState()
	state.Board[Hearts] = SuitSequence{Low: Six, High: Eight}
	state.Board[Diamonds] = SuitSequence{Low: Six, High: Eight}
	state.Hands[0] = []Card{
		{Suit: Hearts, Rank: Nine},
		{Suit: Diamonds, Rank: Nine},
	}
	state.Hands[1] = []Card{{Suit: Clubs, Rank: Two}}

	move, ok := HardStrategy{}.ChooseMove(state, 0)
	if !ok {
		t.Fatal("expected a bot move")
	}
	if move.FaceDown || move.Close {
		t.Fatalf("expected a normal play, got %+v", move)
	}
	if move.Card.Rank != Nine {
		t.Fatalf("hard picked %+v, want a sequence-extending nine", move.Card)
	}
}

func TestStrategiesAreDeterministic(t *testing.T) {
	build := func() GameState {
		state := NewGameState()
		state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
		state.Board[Hearts] = SuitSequence{Low: Six, High: Eight}
		state.Hands[0] = []Card{
			{Suit: Spades, Rank: Five},
			{Suit: Spades, Rank: Nine},
			{Suit: Hearts, Rank: Nine},
		}
		state.Hands[1] = []Card{{Suit: Clubs, Rank: Two}}
		return state
	}
	for _, d := range []BotDifficulty{BotEasy, BotMedium, BotHard} {
		first, ok1 := PickMoveWithDifficulty(build(), 0, d)
		second, ok2 := PickMoveWithDifficulty(build(), 0, d)
		if ok1 != ok2 || !reflect.DeepEqual(first, second) {
			t.Fatalf("difficulty %q non-deterministic: %+v(%t) vs %+v(%t)", d, first, ok1, second, ok2)
		}
	}
}

func TestChooseMoveRejectsInvalidPlayerOrEmptyHand(t *testing.T) {
	state := NewGameState()
	state.Board[Spades] = SuitSequence{Low: Six, High: Eight}
	// seat 0 has no cards
	for _, d := range []BotDifficulty{BotEasy, BotMedium, BotHard} {
		if _, ok := PickMoveWithDifficulty(state, 0, d); ok {
			t.Errorf("difficulty %q returned a move for an empty hand", d)
		}
		if _, ok := PickMoveWithDifficulty(state, -1, d); ok {
			t.Errorf("difficulty %q returned a move for an invalid index", d)
		}
		if _, ok := PickMoveWithDifficulty(state, PlayerCount, d); ok {
			t.Errorf("difficulty %q returned a move for an out-of-range index", d)
		}
	}
}
