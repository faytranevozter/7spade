package game

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
)

const PlayerCount = 4

type ScoringMode string

const (
	ScoringRankValue ScoringMode = "rank_value"
	ScoringFlat      ScoringMode = "flat"
	ScoringCustom    ScoringMode = "custom"
)

type TeamMode string

const (
	TeamFFA TeamMode = "ffa"
	Team2v2 TeamMode = "2v2"
)

type GameConfig struct {
	PlayerCount  int            `json:"player_count"`
	DeckCount    int            `json:"deck_count"`
	ScoringMode  ScoringMode    `json:"scoring_mode"`
	CustomScores map[Rank]int   `json:"custom_scores,omitempty"`
	TeamMode     TeamMode       `json:"team_mode"`
	Teams        [][]int        `json:"teams,omitempty"`
	StartingSuit Suit           `json:"starting_suit"`
}

func DefaultConfig() GameConfig {
	return GameConfig{
		PlayerCount:  PlayerCount,
		DeckCount:    1,
		ScoringMode:  ScoringRankValue,
		TeamMode:     TeamFFA,
		StartingSuit: Spades,
	}
}

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
	Hands         [][]Card
	Board         map[Suit]SuitSequence
	FaceDown      [][]Card
	CurrentPlayer int
	Closed        map[Suit]bool
	CloseMethod   CloseMethod
	Config        GameConfig
}

type MoveOptions struct {
	Cards        []Card
	AceCloses    []AceCloseOption
	FaceDownOnly bool
}

// AceCloseOption describes a suit the player can close with an Ace they hold,
// and which ends (low/high) are currently legal. CanLow requires the sequence
// to reach 2; CanHigh requires it to reach King. When the global CloseMethod is
// already locked, only the matching end is reported.
type AceCloseOption struct {
	Suit    Suit
	CanLow  bool
	CanHigh bool
}

var suits = []Suit{Spades, Hearts, Diamonds, Clubs}

func NewGameState() GameState {
	return NewGameStateWithConfig(DefaultConfig())
}

func NewGameStateWithConfig(cfg GameConfig) GameState {
	if cfg.PlayerCount <= 0 {
		cfg.PlayerCount = PlayerCount
	}
	if cfg.DeckCount <= 0 {
		cfg.DeckCount = 1
	}
	if cfg.ScoringMode == "" {
		cfg.ScoringMode = ScoringRankValue
	}
	if cfg.TeamMode == "" {
		cfg.TeamMode = TeamFFA
	}
	if cfg.StartingSuit == "" {
		cfg.StartingSuit = Spades
	}
	return GameState{
		Hands:    make([][]Card, cfg.PlayerCount),
		FaceDown: make([][]Card, cfg.PlayerCount),
		Board:    map[Suit]SuitSequence{},
		Closed:   map[Suit]bool{},
		Config:   cfg,
	}
}

func Deal(seed int64) (GameState, int) {
	return DealWithConfig(seed, DefaultConfig())
}

func DealWithConfig(seed int64, cfg GameConfig) (GameState, int) {
	deckSize := 52 * cfg.DeckCount
	deck := make([]Card, 0, deckSize)
	for d := 0; d < cfg.DeckCount; d++ {
		for _, suit := range suits {
			for rank := Two; rank <= Ace; rank++ {
				deck = append(deck, Card{Suit: suit, Rank: rank})
			}
		}
	}

	rand.New(rand.NewSource(seed)).Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	state := NewGameStateWithConfig(cfg)
	starter := -1
	for index, card := range deck {
		player := index % cfg.PlayerCount
		state.Hands[player] = append(state.Hands[player], card)
		if starter == -1 && card == (Card{Suit: cfg.StartingSuit, Rank: Seven}) {
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
	aceCloses := AceCloseOptions(state, playerHand)
	// A face-down penalty is only forced when there is no legal play AND no
	// Ace the player could legally close with.
	faceDownOnly := len(moves) == 0 && len(aceCloses) == 0
	return MoveOptions{Cards: moves, AceCloses: aceCloses, FaceDownOnly: faceDownOnly}
}

// AceCloseOptions reports the suits the player can close with an Ace they hold.
// A suit is closable when it is on the board, not already closed, and the
// player holds its Ace. CanLow requires the sequence low to be 2; CanHigh
// requires the sequence high to be King. When the global close method is
// already locked, only the matching end is reported.
func AceCloseOptions(state GameState, playerHand []Card) []AceCloseOption {
	options := make([]AceCloseOption, 0, len(suits))
	for _, suit := range suits {
		if !containsCard(playerHand, Card{Suit: suit, Rank: Ace}) {
			continue
		}
		sequence, started := state.Board[suit]
		if !started || state.Closed[suit] {
			continue
		}
		canLow := sequence.Low == Two
		canHigh := sequence.High == King
		// Honour the locked global method: once set, only that end is legal.
		switch state.CloseMethod {
		case CloseLow:
			canHigh = false
		case CloseHigh:
			canLow = false
		}
		if !canLow && !canHigh {
			continue
		}
		options = append(options, AceCloseOption{Suit: suit, CanLow: canLow, CanHigh: canHigh})
	}
	return options
}

func ApplyMove(state GameState, playerIndex int, card Card, faceDown bool) (GameState, error) {
	if playerIndex < 0 || playerIndex >= len(state.Hands) {
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
		return finalizeIfStalemate(updated), nil
	}

	if !containsCard(moves.Cards, card) {
		return GameState{}, fmt.Errorf("card %+v is not playable", card)
	}
	updated := cloneState(state)
	updated.Hands[playerIndex] = removeCard(updated.Hands[playerIndex], card)
	updated.Board[card.Suit] = applyCardToSequence(updated.Board[card.Suit], card)
	updated.CurrentPlayer = nextPlayerWithCards(updated, playerIndex)
	return finalizeIfStalemate(updated), nil
}

func IsGameOver(state GameState) bool {
	for _, hand := range state.Hands {
		if len(hand) > 0 {
			return false
		}
	}
	return true
}

// isStalemate reports whether the game has reached a dead state: at least one
// player still holds cards, but NO player with cards has any legal action —
// neither a sequence play / new-7, nor a legal Ace close. Because a face-down
// placement only moves a card from a hand into that player's penalty pile and
// never changes the board, such a state is irreversible: every remaining hand
// card is guaranteed to become a face-down penalty. Detecting it lets the game
// end immediately instead of grinding through forced face-down turns.
//
// The check is global on purpose. A single stuck player is a normal forced
// face-down (play continues to whoever can still move); only when EVERY player
// is stuck is the outcome fixed.
//
// Unexported: callers should go through finalizeIfStalemate, which gates the
// sweep on this check.
func isStalemate(state GameState) bool {
	if IsGameOver(state) {
		return false
	}
	for _, hand := range state.Hands {
		if len(hand) == 0 {
			continue
		}
		moves := ValidMoves(state, hand)
		if len(moves.Cards) > 0 || len(moves.AceCloses) > 0 {
			return false
		}
	}
	return true
}

// finalizeStalemate sweeps every remaining hand card into that player's
// face-down pile, emptying all hands. After this, IsGameOver is true and the
// leftover cards are scored as penalties — equivalent to playing the dead state
// out (they would all become face-down anyway), without the busywork.
//
// Unexported and only meant to run on a confirmed stalemate (see
// finalizeIfStalemate); it does not itself check that the state is dead.
func finalizeStalemate(state GameState) GameState {
	updated := cloneState(state)
	for player := range updated.Hands {
		if len(updated.Hands[player]) == 0 {
			continue
		}
		updated.FaceDown[player] = append(updated.FaceDown[player], updated.Hands[player]...)
		updated.Hands[player] = nil
	}
	return updated
}

func CalculateScores(state GameState) []int {
	scores := make([]int, len(state.FaceDown))
	for player, cards := range state.FaceDown {
		for _, card := range cards {
			scores[player] += scoreCard(card, state)
		}
	}
	if state.Config.TeamMode == Team2v2 && len(state.Config.Teams) > 0 {
		teamScores := make(map[int]int)
		playerTeam := make(map[int]int)
		for teamIdx, members := range state.Config.Teams {
			for _, member := range members {
				playerTeam[member] = teamIdx
			}
		}
		for player, score := range scores {
			teamScores[playerTeam[player]] += score
		}
		for player := range scores {
			scores[player] = teamScores[playerTeam[player]]
		}
	}
	return scores
}

func ScoreCard(card Card, state GameState) int {
	return scoreCard(card, state)
}

func scoreCard(card Card, state GameState) int {
	switch state.Config.ScoringMode {
	case ScoringFlat:
		return 1
	case ScoringCustom:
		if state.Config.CustomScores != nil {
			if v, ok := state.Config.CustomScores[card.Rank]; ok {
				return v
			}
		}
		return aceAdjustedValue(card, state.CloseMethod)
	default:
		return aceAdjustedValue(card, state.CloseMethod)
	}
}

func aceAdjustedValue(card Card, method CloseMethod) int {
	if card.Rank == Ace {
		switch method {
		case CloseLow:
			return 1
		case CloseHigh:
			return int(Ace)
		default:
			// No Ace closed a suit (high or low) all game: a dangling Ace is
			// scored as a Seven rather than its full rank.
			return int(Seven)
		}
	}
	return card.PointValue()
}

func ApplyAceClose(state GameState, playerIndex int, suit Suit, method CloseMethod) (GameState, error) {
	if playerIndex < 0 || playerIndex >= len(state.Hands) {
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
	return finalizeIfStalemate(updated), nil
}

// finalizeIfStalemate sweeps remaining hands into face-down piles when the game
// has reached an irreversible no-playables state, so callers see IsGameOver via
// the normal path. A no-op when the game can still progress or is already over.
func finalizeIfStalemate(state GameState) GameState {
	if isStalemate(state) {
		return finalizeStalemate(state)
	}
	return state
}

func isPlayable(state GameState, card Card) bool {
	if state.Closed[card.Suit] {
		return false
	}
	if card.Rank == Ace {
		return false
	}
	sequence, started := state.Board[card.Suit]
	if !started {
		if boardIsEmpty(state.Board) {
			return card == (Card{Suit: state.Config.StartingSuit, Rank: Seven})
		}
		return card.Rank == Seven
	}
	if card.Rank == sequence.Low-1 || card.Rank == sequence.High+1 {
		return true
	}
	if state.Config.DeckCount > 1 {
		return card.Rank >= sequence.Low && card.Rank <= sequence.High
	}
	return false
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
	playerCount := len(state.Hands)
	for offset := 1; offset <= playerCount; offset++ {
		candidate := (current + offset) % playerCount
		if len(state.Hands[candidate]) > 0 {
			return candidate
		}
	}
	return current
}

func cloneState(state GameState) GameState {
	clone := GameState{
		Hands:         make([][]Card, len(state.Hands)),
		FaceDown:      make([][]Card, len(state.FaceDown)),
		Board:         make(map[Suit]SuitSequence, len(state.Board)),
		CurrentPlayer: state.CurrentPlayer,
		Closed:        make(map[Suit]bool, len(state.Closed)),
		CloseMethod:   state.CloseMethod,
		Config:        state.Config,
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
