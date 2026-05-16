package game

type BotMove struct {
	Card     Card
	FaceDown bool
}

func PickMove(state GameState, playerHand []Card) (BotMove, bool) {
	if len(playerHand) == 0 {
		return BotMove{}, false
	}

	moves := ValidMoves(state, playerHand)
	if len(moves.Cards) > 0 {
		return BotMove{Card: moves.Cards[0]}, true
	}
	return BotMove{Card: playerHand[0], FaceDown: true}, true
}
