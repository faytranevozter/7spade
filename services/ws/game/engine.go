package game

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
)

const PlayerCount = 4

type Suit string

const (
	Spades   Suit = "spades"
	Hearts   Suit = "hearts"
	Diamonds Suit = "diamonds"
	Clubs    Suit = "clubs"
)

type Rank int

const (
	Two   Rank = 2
	Three Rank = 3
	Four  Rank = 4
	Five  Rank = 5
	Six   Rank = 6
	Seven Rank = 7
	Eight Rank = 8
	Nine  Rank = 9
	Ten   Rank = 10
	Jack  Rank = 11
	Queen Rank = 12
	King  Rank = 13
	Ace   Rank = 14
)

type Card struct {
	Suit Suit
	Rank Rank
}

func (c Card) PointValue() int {
	return int(c.Rank)
}

type CloseMethod string

const (
	CloseLow  CloseMethod = "low"
	CloseHigh CloseMethod = "high"
)

type SuitSequence struct {
	Low  Rank
	High Rank
}

type GameState struct {
	Hands         [PlayerCount][]Card
	Board         map[Suit]SuitSequence
	FaceDown      [PlayerCount][]Card
	CurrentPlayer int
	Closed        map[Suit]bool
	CloseMethod   CloseMethod
}

type MoveOptions struct {
	Cards        []Card
	FaceDownOnly bool
}

var suits = []Suit{Spades, Hearts, Diamonds, Clubs}

func NewGameState() GameState {
	return GameState{Board: map[Suit]SuitSequence{}, Closed: map[Suit]bool{}}
}

func Deal(seed int64) (GameState, int) {
	deck := make([]Card, 0, 52)
	for _, suit := range suits {
		for rank := Two; rank <= Ace; rank++ {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}

	rand.New(rand.NewSource(seed)).Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	state := NewGameState()
	starter := -1
	for index, card := range deck {
		player := index % PlayerCount
		state.Hands[player] = append(state.Hands[player], card)
		if card == (Card{Suit: Spades, Rank: Seven}) {
			starter = player
		}
	}
	for player := range state.Hands {
		sortCards(state.Hands[player])
	}
	state.CurrentPlayer = starter

	return state, starter
}

func ValidMoves(state GameState, playerHand []Card) MoveOptions {
	moves := make([]Card, 0, len(playerHand))
	for _, card := range playerHand {
		if isPlayable(state, card) {
			moves = append(moves, card)
		}
	}
	return MoveOptions{Cards: moves, FaceDownOnly: len(moves) == 0}
}

func ApplyMove(state GameState, playerIndex int, card Card, faceDown bool) (GameState, error) {
	if playerIndex < 0 || playerIndex >= PlayerCount {
		return GameState{}, fmt.Errorf("player index %d out of range", playerIndex)
	}
	if !containsCard(state.Hands[playerIndex], card) {
		return GameState{}, fmt.Errorf("player %d does not hold card %+v", playerIndex, card)
	}

	moves := ValidMoves(state, state.Hands[playerIndex])
	if faceDown {
		if !moves.FaceDownOnly {
			return GameState{}, errors.New("cannot place face-down while a legal play is available")
		}
		updated := cloneState(state)
		updated.Hands[playerIndex] = removeCard(updated.Hands[playerIndex], card)
		updated.FaceDown[playerIndex] = append(updated.FaceDown[playerIndex], card)
		updated.CurrentPlayer = nextPlayerWithCards(updated, playerIndex)
		return updated, nil
	}

	if !containsCard(moves.Cards, card) {
		return GameState{}, fmt.Errorf("card %+v is not playable", card)
	}
	updated := cloneState(state)
	updated.Hands[playerIndex] = removeCard(updated.Hands[playerIndex], card)
	updated.Board[card.Suit] = applyCardToSequence(updated.Board[card.Suit], card)
	updated.CurrentPlayer = nextPlayerWithCards(updated, playerIndex)
	return updated, nil
}

func IsGameOver(state GameState) bool {
	for _, hand := range state.Hands {
		if len(hand) > 0 {
			return false
		}
	}
	return true
}

func CalculateScores(state GameState) [PlayerCount]int {
	var scores [PlayerCount]int
	for player, cards := range state.FaceDown {
		for _, card := range cards {
			scores[player] += aceAdjustedValue(card, state.CloseMethod)
		}
	}
	return scores
}

func aceAdjustedValue(card Card, method CloseMethod) int {
	if card.Rank == Ace && method == CloseLow {
		return 1
	}
	return card.PointValue()
}

func ApplyAceClose(state GameState, playerIndex int, suit Suit, method CloseMethod) (GameState, error) {
	if playerIndex < 0 || playerIndex >= PlayerCount {
		return GameState{}, fmt.Errorf("player index %d out of range", playerIndex)
	}

	sequence, started := state.Board[suit]
	if !started {
		return GameState{}, fmt.Errorf("suit %s is not on the board", suit)
	}
	if state.Closed[suit] {
		return GameState{}, fmt.Errorf("suit %s is already closed", suit)
	}

	switch method {
	case CloseLow:
		if sequence.Low != Two {
			return GameState{}, fmt.Errorf("cannot close %s low: sequence low is %d, need 2", suit, sequence.Low)
		}
	case CloseHigh:
		if sequence.High != King {
			return GameState{}, fmt.Errorf("cannot close %s high: sequence high is %d, need K", suit, sequence.High)
		}
	default:
		return GameState{}, fmt.Errorf("unknown close method: %s", method)
	}

	aceCard := Card{Suit: suit, Rank: Ace}
	if !containsCard(state.Hands[playerIndex], aceCard) {
		return GameState{}, fmt.Errorf("player %d does not hold the ace of %s", playerIndex, suit)
	}

	if state.CloseMethod != "" && state.CloseMethod != method {
		return GameState{}, fmt.Errorf("close method already locked to %s, cannot use %s", state.CloseMethod, method)
	}

	updated := cloneState(state)
	updated.Hands[playerIndex] = removeCard(updated.Hands[playerIndex], aceCard)
	updated.Closed[suit] = true
	if updated.CloseMethod == "" {
		updated.CloseMethod = method
	}
	updated.CurrentPlayer = nextPlayerWithCards(updated, playerIndex)
	return updated, nil
}

func isPlayable(state GameState, card Card) bool {
	if state.Closed[card.Suit] {
		return false
	}
	sequence, started := state.Board[card.Suit]
	if !started {
		if boardIsEmpty(state.Board) {
			return card == (Card{Suit: Spades, Rank: Seven})
		}
		return card.Rank == Seven
	}
	return card.Rank == sequence.Low-1 || card.Rank == sequence.High+1
}

func applyCardToSequence(sequence SuitSequence, card Card) SuitSequence {
	if sequence == (SuitSequence{}) {
		return SuitSequence{Low: card.Rank, High: card.Rank}
	}
	if card.Rank < sequence.Low {
		sequence.Low = card.Rank
	}
	if card.Rank > sequence.High {
		sequence.High = card.Rank
	}
	return sequence
}

func boardIsEmpty(board map[Suit]SuitSequence) bool {
	return len(board) == 0
}

func nextPlayerWithCards(state GameState, current int) int {
	if IsGameOver(state) {
		return current
	}
	for offset := 1; offset <= PlayerCount; offset++ {
		candidate := (current + offset) % PlayerCount
		if len(state.Hands[candidate]) > 0 {
			return candidate
		}
	}
	return current
}

func cloneState(state GameState) GameState {
	clone := GameState{
		Board:         make(map[Suit]SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
	}
	for player := range state.Hands {
		clone.Hands[player] = append([]Card(nil), state.Hands[player]...)
		clone.FaceDown[player] = append([]Card(nil), state.FaceDown[player]...)
	}
	for suit, sequence := range state.Board {
		clone.Board[suit] = sequence
	}
	for suit, closed := range state.Closed {
		clone.Closed[suit] = closed
	}
	return clone
}

func containsCard(cards []Card, target Card) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}

func removeCard(cards []Card, target Card) []Card {
	for index, card := range cards {
		if card == target {
			return append(cards[:index], cards[index+1:]...)
		}
	}
	return cards
}

func sortCards(cards []Card) {
	suitOrder := map[Suit]int{Spades: 0, Hearts: 1, Diamonds: 2, Clubs: 3}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Suit != cards[j].Suit {
			return suitOrder[cards[i].Suit] < suitOrder[cards[j].Suit]
		}
		return cards[i].Rank < cards[j].Rank
	})
}
