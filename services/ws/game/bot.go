package game

// BotMove is a single decision produced by a Strategy: either a normal/face-down
// card placement, or an Ace close (Close=true, Method set, Card is the Ace).
type BotMove struct {
	Card     Card
	FaceDown bool
	// Close indicates this move closes a suit with an Ace via ApplyAceClose.
	// When true, Method is the close method to use and Card is the Ace.
	Close  bool
	Method CloseMethod
}

type BotDifficulty string

const (
	BotEasy   BotDifficulty = "easy"
	BotMedium BotDifficulty = "medium"
	BotHard   BotDifficulty = "hard"
)

// Strategy chooses a bot move from the current game state for a given seat.
//
// ChooseMove takes the player INDEX (not a loose hand) so strategies can read
// public state — opponent hand counts, the board, closed suits, the locked close
// method, and their own face-down pile — and infer the unknown-card universe.
// This differs deliberately from the issue's `ChoosePlay(board, hand)` wording,
// which cannot support opponent-aware inference: a bare hand has no link back to
// seat position or opponent counts. See docs/specs/bot-difficulty.md.
type Strategy interface {
	ChooseMove(state GameState, playerIndex int) (BotMove, bool)
}

type EasyStrategy struct{}
type MediumStrategy struct{}
type HardStrategy struct{}

// StrategyFor maps a difficulty to its Strategy. Unknown/empty difficulties fall
// back to medium, matching the WS service's default for bot backfill.
func StrategyFor(difficulty BotDifficulty) Strategy {
	switch difficulty {
	case BotEasy:
		return EasyStrategy{}
	case BotHard:
		return HardStrategy{}
	default:
		return MediumStrategy{}
	}
}

// PickMoveWithDifficulty is the WS entry point: pick a move for playerIndex using
// the difficulty's strategy.
func PickMoveWithDifficulty(state GameState, playerIndex int, difficulty BotDifficulty) (BotMove, bool) {
	return StrategyFor(difficulty).ChooseMove(state, playerIndex)
}

// validForPlayer guards index bounds and an empty hand, returning the hand and
// move options when the seat can act.
func validForPlayer(state GameState, playerIndex int) ([]Card, MoveOptions, bool) {
	if playerIndex < 0 || playerIndex >= len(state.Hands) {
		return nil, MoveOptions{}, false
	}
	hand := state.Hands[playerIndex]
	if len(hand) == 0 {
		return nil, MoveOptions{}, false
	}
	return hand, ValidMoves(state, hand), true
}

// firstAceClose builds the close move for the first legal Ace option, honouring
// the locked close method.
func firstAceClose(state GameState, option AceCloseOption) BotMove {
	method := pickCloseMethod(state.CloseMethod, option)
	return BotMove{Card: Card{Suit: option.Suit, Rank: Ace}, Close: true, Method: method}
}

// ---------------------------------------------------------------------------
// Scoring weights. Integer weights keep scoring deterministic and easy to reason
// about; tests assert on relative behaviour, not exact totals.
// ---------------------------------------------------------------------------
const (
	progressWeight            = 20
	penaltyWeight             = 2
	opponentBenefitWeight     = 15
	defensiveWeight           = 10
	futureMoveWeight          = 12
	aceCloseSelfBenefitWeight = 10
)

// ---------------------------------------------------------------------------
// EasyStrategy: original deterministic behaviour. First valid play, then prefer
// an Ace close over a face-down penalty, otherwise drop the first hand card.
// No board analysis, no lookahead.
// ---------------------------------------------------------------------------
func (EasyStrategy) ChooseMove(state GameState, playerIndex int) (BotMove, bool) {
	hand, moves, ok := validForPlayer(state, playerIndex)
	if !ok {
		return BotMove{}, false
	}
	if len(moves.Cards) > 0 {
		return BotMove{Card: moves.Cards[0]}, true
	}
	if len(moves.AceCloses) > 0 {
		return firstAceClose(state, moves.AceCloses[0]), true
	}
	return BotMove{Card: hand[0], FaceDown: true}, true
}

// ---------------------------------------------------------------------------
// MediumStrategy: board- and opponent-aware. Prefers plays that extend a suit
// closer to completion, avoids closing suits that benefit opponents, and throws
// the lowest-penalty card when forced face-down.
// ---------------------------------------------------------------------------
func (MediumStrategy) ChooseMove(state GameState, playerIndex int) (BotMove, bool) {
	hand, moves, ok := validForPlayer(state, playerIndex)
	if !ok {
		return BotMove{}, false
	}

	bestCard, hasCard := bestMediumCard(state, playerIndex, moves.Cards)

	// No legal play: close an Ace before taking a face-down penalty.
	if !hasCard {
		if len(moves.AceCloses) > 0 {
			return firstAceClose(state, moves.AceCloses[0]), true
		}
		return BotMove{Card: lowestPenaltyCard(hand, state.CloseMethod), FaceDown: true}, true
	}

	// A legal play exists. Only close an Ace instead if the close is clearly
	// better and does not hand opponents an advantage.
	if len(moves.AceCloses) > 0 {
		close := moves.AceCloses[0]
		closeMove := firstAceClose(state, close)
		cardScore := mediumCardScore(state, playerIndex, bestCard)
		closeScore := -opponentBenefitWeight * opponentBenefitScore(state, playerIndex, closeMove)
		if closeScore > cardScore {
			return closeMove, true
		}
	}
	return BotMove{Card: bestCard}, true
}

// bestMediumCard returns the highest-scoring playable card, with a deterministic
// tie-break. The bool is false when there are no playable cards.
func bestMediumCard(state GameState, playerIndex int, cards []Card) (Card, bool) {
	if len(cards) == 0 {
		return Card{}, false
	}
	best := cards[0]
	bestScore := mediumCardScore(state, playerIndex, best)
	for _, card := range cards[1:] {
		score := mediumCardScore(state, playerIndex, card)
		if score > bestScore || (score == bestScore && stableCardTieBreak(card, best)) {
			best, bestScore = card, score
		}
	}
	return best, true
}

func mediumCardScore(state GameState, playerIndex int, card Card) int {
	score := progressWeight * sequenceProgressAfter(state, card)
	score -= penaltyWeight * aceAdjustedValue(card, state.CloseMethod)
	score -= opponentBenefitWeight * opponentBenefitScore(state, playerIndex, BotMove{Card: card})
	return score
}

// ---------------------------------------------------------------------------
// HardStrategy: full board analysis over the inferred unknown-card universe.
// Maximises future flexibility, plays defensively, delays Ace closes that would
// benefit opponents, and discards the lowest expected-risk card face-down.
// ---------------------------------------------------------------------------
func (HardStrategy) ChooseMove(state GameState, playerIndex int) (BotMove, bool) {
	hand, moves, ok := validForPlayer(state, playerIndex)
	if !ok {
		return BotMove{}, false
	}

	bestCard, cardScore, hasCard := bestHardCard(state, playerIndex, moves.Cards)

	if len(moves.AceCloses) > 0 {
		close := moves.AceCloses[0]
		closeMove := firstAceClose(state, close)
		closeScore := aceCloseSelfBenefitWeight*aceCloseSelfBenefit(state, playerIndex, close) -
			opponentBenefitWeight*opponentBenefitScore(state, playerIndex, closeMove)
		// Delay the close when a normal play exists and scores at least as well.
		if !hasCard || closeScore > cardScore {
			return closeMove, true
		}
		return BotMove{Card: bestCard}, true
	}

	if hasCard {
		return BotMove{Card: bestCard}, true
	}

	// Forced face-down: discard the lowest expected-risk card.
	return BotMove{Card: lowestRiskCard(state, playerIndex, hand), FaceDown: true}, true
}

func bestHardCard(state GameState, playerIndex int, cards []Card) (Card, int, bool) {
	if len(cards) == 0 {
		return Card{}, 0, false
	}
	best := cards[0]
	bestScore := hardCardScore(state, playerIndex, best)
	for _, card := range cards[1:] {
		score := hardCardScore(state, playerIndex, card)
		if score > bestScore || (score == bestScore && stableCardTieBreak(card, best)) {
			best, bestScore = card, score
		}
	}
	return best, bestScore, true
}

func hardCardScore(state GameState, playerIndex int, card Card) int {
	move := BotMove{Card: card}
	score := progressWeight * sequenceProgressAfter(state, card)
	score -= penaltyWeight * aceAdjustedValue(card, state.CloseMethod)
	score -= opponentBenefitWeight * opponentBenefitScore(state, playerIndex, move)
	score += defensiveWeight * defensiveBlockScore(state, playerIndex, move)
	if sim, ok := applyCandidateForScoring(state, playerIndex, move); ok {
		score += futureMoveWeight * futureLegalMoveCount(sim, playerIndex)
	}
	return score
}

// lowestRiskCard chooses the safest face-down discard: the card with the lowest
// expectedFacedownRisk, tie-broken deterministically.
func lowestRiskCard(state GameState, playerIndex int, hand []Card) Card {
	best := hand[0]
	bestRisk := expectedFacedownRisk(state, playerIndex, best)
	for _, card := range hand[1:] {
		risk := expectedFacedownRisk(state, playerIndex, card)
		if risk < bestRisk || (risk == bestRisk && stableCardTieBreak(card, best)) {
			best, bestRisk = card, risk
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Retained primitives.
// ---------------------------------------------------------------------------

// sequenceProgressAfter returns the length the card's suit sequence would reach
// if the card were played. A brand-new suit scores 1.
func sequenceProgressAfter(state GameState, card Card) int {
	sequence, started := state.Board[card.Suit]
	if !started {
		return 1
	}
	if card.Rank < sequence.Low {
		sequence.Low = card.Rank
	}
	if card.Rank > sequence.High {
		sequence.High = card.Rank
	}
	return int(sequence.High - sequence.Low + 1)
}

func lowestPenaltyCard(cards []Card, closeMethod CloseMethod) Card {
	selected := cards[0]
	for _, card := range cards[1:] {
		value := aceAdjustedValue(card, closeMethod)
		if value < aceAdjustedValue(selected, closeMethod) ||
			(value == aceAdjustedValue(selected, closeMethod) && stableCardTieBreak(card, selected)) {
			selected = card
		}
	}
	return selected
}

// pickCloseMethod resolves which end to close. A locked global method wins;
// otherwise prefer low (the Ace then counts as 1 instead of 14, minimising
// penalty risk for everyone still holding that Ace).
func pickCloseMethod(locked CloseMethod, option AceCloseOption) CloseMethod {
	switch locked {
	case CloseLow:
		return CloseLow
	case CloseHigh:
		return CloseHigh
	}
	if option.CanLow {
		return CloseLow
	}
	return CloseHigh
}

// ---------------------------------------------------------------------------
// No-cheating inference model.
//
// Strategies may only reason about PUBLIC information plus their own private
// cards. They must never inspect opponent hand contents or opponent face-down
// identities. knownCards is therefore: the bot's own hand, every card on the
// board, the Ace of each closed suit (closing removes the Ace from a hand and
// sets Closed[suit]=true), and the bot's own face-down pile. unknownCards is the
// remainder of the deck — which legitimately still contains opponents' real
// cards, because the bot is not allowed to know them.
// ---------------------------------------------------------------------------

func fullDeck() []Card {
	return fullDeckWithConfig(DefaultConfig())
}

func fullDeckWithConfig(cfg GameConfig) []Card {
	deckSize := 52 * cfg.DeckCount
	deck := make([]Card, 0, deckSize)
	for d := 0; d < cfg.DeckCount; d++ {
		for _, suit := range suits {
			for rank := Two; rank <= Ace; rank++ {
				deck = append(deck, Card{Suit: suit, Rank: rank})
			}
		}
	}
	return deck
}

// boardCards expands every started suit sequence from Low to High.
func boardCards(state GameState) []Card {
	cards := make([]Card, 0, 16)
	for suit, sequence := range state.Board {
		for rank := sequence.Low; rank <= sequence.High; rank++ {
			cards = append(cards, Card{Suit: suit, Rank: rank})
		}
	}
	return cards
}

// knownCardCounts returns how many copies of each card the bot can account for
// (own hand, own face-down, board, closed Aces). Counts, not a set — multi-deck
// games can have several visible copies of the same rank/suit.
func knownCardCounts(state GameState, playerIndex int) map[Card]int {
	known := make(map[Card]int, 52*state.Config.DeckCount)
	if playerIndex >= 0 && playerIndex < len(state.Hands) {
		for _, card := range state.Hands[playerIndex] {
			known[card]++
		}
		for _, card := range state.FaceDown[playerIndex] {
			known[card]++
		}
	}
	for _, card := range boardCards(state) {
		known[card]++
	}
	// A closed suit's Ace is public knowledge: closing consumes it from a hand.
	for suit, closed := range state.Closed {
		if closed {
			known[Card{Suit: suit, Rank: Ace}]++
		}
	}
	return known
}

// knownCards is the set view of knownCardCounts (any copy known ⇒ present).
// Callers that only need presence (e.g. neighbour-slot checks) use this.
func knownCards(state GameState, playerIndex int) map[Card]bool {
	counts := knownCardCounts(state, playerIndex)
	known := make(map[Card]bool, len(counts))
	for card := range counts {
		known[card] = true
	}
	return known
}

// unknownCards returns every deck card the bot cannot account for, WITH
// multiplicity. In a multi-deck game (DeckCount > 1) a card appears
// DeckCount times in the full deck; only the counted known copies are
// subtracted so remaining copies stay in the unknown universe.
func unknownCards(state GameState, playerIndex int) []Card {
	known := knownCardCounts(state, playerIndex)
	deck := fullDeckWithConfig(state.Config)
	deckCount := map[Card]int{}
	for _, card := range deck {
		deckCount[card]++
	}
	unknown := make([]Card, 0, len(deck))
	for card, total := range deckCount {
		knownCopies := known[card]
		if knownCopies > total {
			knownCopies = total
		}
		for i := 0; i < total-knownCopies; i++ {
			unknown = append(unknown, card)
		}
	}
	return unknown
}

// opponentHandCount sums the number of cards held by every seat other than the
// bot. Only counts are used — never card values.
func opponentHandCount(state GameState, playerIndex int) int {
	total := 0
	for i := 0; i < len(state.Hands); i++ {
		if i == playerIndex {
			continue
		}
		total += len(state.Hands[i])
	}
	return total
}

// outstandingCardsOfSuit counts how many cards of a suit appear in the given set
// (typically the unknown-card universe).
func outstandingCardsOfSuit(cards []Card, suit Suit) int {
	count := 0
	for _, card := range cards {
		if card.Suit == suit {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Scoring helpers.
// ---------------------------------------------------------------------------

// applyCandidateForScoring simulates a candidate move and returns the resulting
// state. It never mutates the input (engine functions clone). Returns false for
// illegal candidates.
func applyCandidateForScoring(state GameState, playerIndex int, move BotMove) (GameState, bool) {
	switch {
	case move.Close:
		updated, err := ApplyAceClose(state, playerIndex, move.Card.Suit, move.Method)
		if err != nil {
			return GameState{}, false
		}
		return updated, true
	case move.FaceDown:
		updated, err := ApplyMove(state, playerIndex, move.Card, true)
		if err != nil {
			return GameState{}, false
		}
		return updated, true
	default:
		updated, err := ApplyMove(state, playerIndex, move.Card, false)
		if err != nil {
			return GameState{}, false
		}
		return updated, true
	}
}

// futureLegalMoveCount reports how many legal plays/closes the bot would still
// have in a (already simulated) state. Higher means more flexibility retained.
func futureLegalMoveCount(state GameState, playerIndex int) int {
	if playerIndex < 0 || playerIndex >= len(state.Hands) {
		return 0
	}
	moves := ValidMoves(state, state.Hands[playerIndex])
	return len(moves.Cards) + len(moves.AceCloses)
}

// suitCompletionDistance estimates how many ranks a suit still needs before both
// ends reach their closable bounds (2 low, King high). An unstarted suit returns
// the full span. Smaller means closer to completion.
func suitCompletionDistance(state GameState, suit Suit) int {
	sequence, started := state.Board[suit]
	if !started {
		return int(King - Two)
	}
	distance := 0
	if sequence.Low > Two {
		distance += int(sequence.Low - Two)
	}
	if sequence.High < King {
		distance += int(King - sequence.High)
	}
	return distance
}

// opponentBenefitScore estimates how much a move helps opponents.
//
// A normal sequence play only ever hands opponents the cards immediately beyond
// the new ends (low-1 and high+1); we count how many of those specific neighbour
// ranks are still unaccounted for (i.e. could sit in an opponent hand). That is
// 0, 1, or 2 — extending your own suit is mostly good for you, not them.
//
// Closing a suit is different: it locks the suit and the global close method. A
// close on a wide-open suit with many outstanding cards is the opponent-relevant
// case the issue calls out, so it is scored on outstanding cards plus closeness.
func opponentBenefitScore(state GameState, playerIndex int, move BotMove) int {
	if opponentHandCount(state, playerIndex) == 0 {
		return 0
	}
	if move.Close {
		unknown := unknownCards(state, playerIndex)
		outstanding := outstandingCardsOfSuit(unknown, move.Card.Suit)
		closeness := int(King-Two) - suitCompletionDistance(state, move.Card.Suit)
		if closeness < 0 {
			closeness = 0
		}
		// Closing trims one slot but locks the suit; damp the raw figure.
		return (outstanding + closeness + 1) / 2
	}
	if move.FaceDown {
		return 0
	}
	return newlyOpenedOpponentSlots(state, playerIndex, move.Card)
}

// newlyOpenedOpponentSlots counts how many of the ranks freshly made playable by
// a normal play (the cards just beyond each new end) are still in the unknown
// universe, and so could be held by an opponent.
func newlyOpenedOpponentSlots(state GameState, playerIndex int, card Card) int {
	sequence, started := state.Board[card.Suit]
	low, high := card.Rank, card.Rank
	if started {
		low, high = sequence.Low, sequence.High
		if card.Rank < low {
			low = card.Rank
		}
		if card.Rank > high {
			high = card.Rank
		}
	}
	known := knownCards(state, playerIndex)
	count := 0
	if low-1 >= Two {
		if neighbour := (Card{Suit: card.Suit, Rank: low - 1}); !known[neighbour] {
			count++
		}
	}
	if high+1 <= King {
		if neighbour := (Card{Suit: card.Suit, Rank: high + 1}); !known[neighbour] {
			count++
		}
	}
	return count
}

// defensiveBlockScore rewards plays that deny opponents an easy extension. A
// move that pushes a suit toward a closable bound while opponents still hold
// outstanding cards of that suit is defensively valuable.
func defensiveBlockScore(state GameState, playerIndex int, move BotMove) int {
	if move.FaceDown {
		return 0
	}
	unknown := unknownCards(state, playerIndex)
	outstanding := outstandingCardsOfSuit(unknown, move.Card.Suit)
	if outstanding == 0 {
		return 0
	}
	before := suitCompletionDistance(state, move.Card.Suit)
	sim, ok := applyCandidateForScoring(state, playerIndex, move)
	if !ok {
		return 0
	}
	after := suitCompletionDistance(sim, move.Card.Suit)
	progress := before - after
	if progress < 0 {
		progress = 0
	}
	return progress
}

// aceCloseSelfBenefit measures how much closing helps the bot: closing removes a
// high-value Ace from its own hand (avoiding a likely face-down penalty) and
// shrinks future obligations.
func aceCloseSelfBenefit(state GameState, playerIndex int, option AceCloseOption) int {
	// Closing low keeps the Ace at value 1; closing high leaves it at 14. The
	// self benefit is the penalty avoided by removing the Ace from the hand.
	method := pickCloseMethod(state.CloseMethod, option)
	ace := Card{Suit: option.Suit, Rank: Ace}
	return aceAdjustedValue(ace, method)
}

// expectedFacedownRisk scores how costly a card is to discard face-down. Base is
// its adjusted point value. Dead cards (no outstanding neighbours, so unlikely
// to ever become playable) are SAFER to discard, so their risk is reduced; cards
// that are close to becoming playable are worth keeping, so their risk is
// raised. Lower is safer.
func expectedFacedownRisk(state GameState, playerIndex int, card Card) int {
	risk := aceAdjustedValue(card, state.CloseMethod) * 2
	unknown := unknownCards(state, playerIndex)
	outstanding := outstandingCardsOfSuit(unknown, card.Suit)
	if state.Closed[card.Suit] {
		// A closed suit can never take this card again — pure dead weight, the
		// safest possible discard.
		return risk - 6
	}
	if outstanding == 0 {
		// No neighbours left in play: this card is effectively dead.
		risk -= 4
	}
	// The nearer the suit is to completion, the more likely this card slots in
	// soon, so keeping it is valuable — raise the discard risk.
	closeness := int(King-Two) - suitCompletionDistance(state, card.Suit)
	if closeness > 0 {
		risk += closeness
	}
	return risk
}

// stableCardTieBreak returns true when a should be preferred over b under a
// fixed ordering (suit order, then rank). Guarantees deterministic selection.
func stableCardTieBreak(a, b Card) bool {
	order := map[Suit]int{Spades: 0, Hearts: 1, Diamonds: 2, Clubs: 3}
	if order[a.Suit] != order[b.Suit] {
		return order[a.Suit] < order[b.Suit]
	}
	return a.Rank < b.Rank
}
