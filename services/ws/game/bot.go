package game

type BotMove struct {
	Card     Card
	FaceDown bool
	// Close indicates this move closes a suit with an Ace via ApplyAceClose.
	// When true, Method is the close method to use and Card is the Ace.
	Close  bool
	Method CloseMethod
}

func PickMove(state GameState, playerHand []Card) (BotMove, bool) {
	if len(playerHand) == 0 {
		return BotMove{}, false
	}

	moves := ValidMoves(state, playerHand)
	if len(moves.Cards) > 0 {
		return BotMove{Card: moves.Cards[0]}, true
	}
	// Prefer closing a suit with an Ace over taking a face-down penalty.
	if len(moves.AceCloses) > 0 {
		option := moves.AceCloses[0]
		method := pickCloseMethod(state.CloseMethod, option)
		return BotMove{Card: Card{Suit: option.Suit, Rank: Ace}, Close: true, Method: method}, true
	}
	return BotMove{Card: playerHand[0], FaceDown: true}, true
}

// pickCloseMethod resolves which end to close. A locked global method wins;
// otherwise prefer low (Ace counts as 1 instead of 14, minimising penalty risk
// for everyone holding that Ace later).
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
