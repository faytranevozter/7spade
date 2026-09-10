package room

import (
	"fmt"

	"github.com/faytranevozter/7spade/services/ws/game"
)

func (room *room) handleMessage(player *player, message clientMessage) {
	room.mu.Lock()

	// Lobby-phase messages are handled before the started/turn checks.
	if room.phase == phaseLobby {
		room.mu.Unlock()
		switch message.Type {
		case messageTypeSetReady:
			room.handleSetReady(player, message.Ready)
		case messageTypeStartGame:
			room.handleStartGame(player)
		case messageTypeKick:
			room.handleKick(player, message.Target)
		case messageTypeLeave:
			room.handleLobbyLeave(player)
		case messageTypeSetTeam:
			room.handleSetTeam(player, message.Team)
		case messageTypeEmote:
			room.handleEmote(player, message.Emote)
		default:
			player.sendError("game has not started")
		}
		return
	}

	if !room.started {
		room.mu.Unlock()
		player.sendError("game has not started")
		return
	}
	if player.disconnected {
		room.mu.Unlock()
		player.sendError("player is disconnected")
		return
	}
	if message.Type == messageTypeRematchVote {
		notify := room.handleRematchVoteLocked(player)
		room.mu.Unlock()
		notify()
		return
	}
	if message.Type == messageTypeGoToWaitingRoom {
		notify := room.handleGoToWaitingRoomLocked(player)
		room.mu.Unlock()
		notify()
		return
	}
	// Emotes are social, not gameplay: they're allowed on any player's turn, so
	// handle them before the turn-ownership check. handleEmote manages its own
	// locking, so release the lock first.
	if message.Type == messageTypeEmote {
		room.mu.Unlock()
		room.handleEmote(player, message.Emote)
		return
	}
	if room.state.CurrentPlayer != player.index {
		room.mu.Unlock()
		player.sendError("not your turn")
		return
	}

	state, move, err := applyClientMessage(room.state, player.index, message)
	if err != nil {
		room.mu.Unlock()
		player.sendError(err.Error())
		return
	}
	room.state = state
	room.moves = append(room.moves, move)
	room.persistLocked()
	gameOver := game.IsGameOver(room.state)
	if !gameOver {
		room.startTurnTimerLocked()
	}
	room.mu.Unlock()

	if gameOver {
		room.saveGameResult()
		room.broadcastGameOver()
		return
	}
	room.broadcastState()
	room.playBotIfNeeded()
}

func applyClientMessage(state game.GameState, playerIndex int, message clientMessage) (game.GameState, recordedMove, error) {
	switch message.Type {
	case messageTypePlayCard:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		if card.Rank == game.Ace {
			method, err := resolveAceCloseMethod(state, playerIndex, card.Suit, message.Method)
			if err != nil {
				return game.GameState{}, recordedMove{}, err
			}
			newState, err := game.ApplyAceClose(state, playerIndex, card.Suit, method)
			if err != nil {
				return game.GameState{}, recordedMove{}, err
			}
			return newState, recordedMove{
				PlayerIndex:  playerIndex,
				Suit:         card.Suit,
				Rank:         card.Rank,
				Type:         moveTypeAceClose,
				AceDirection: method,
			}, nil
		}
		newState, err := game.ApplyMove(state, playerIndex, card, false)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex: playerIndex,
			Suit:        card.Suit,
			Rank:        card.Rank,
			Type:        moveTypePlay,
		}, nil
	case messageTypePlaceFaceDown:
		card, err := parseCard(message.Suit, message.Rank)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		newState, err := game.ApplyMove(state, playerIndex, card, true)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex: playerIndex,
			Suit:        card.Suit,
			Rank:        card.Rank,
			Type:        moveTypeFaceDown,
		}, nil
	default:
		return game.GameState{}, recordedMove{}, fmt.Errorf("unknown message type: %s", message.Type)
	}
}

func applyBotMove(state game.GameState, playerIndex int, move game.BotMove) (game.GameState, recordedMove, error) {
	if move.Close {
		newState, err := game.ApplyAceClose(state, playerIndex, move.Card.Suit, move.Method)
		if err != nil {
			return game.GameState{}, recordedMove{}, err
		}
		return newState, recordedMove{
			PlayerIndex:  playerIndex,
			Suit:         move.Card.Suit,
			Rank:         move.Card.Rank,
			Type:         moveTypeAceClose,
			AceDirection: move.Method,
		}, nil
	}
	moveType := moveTypePlay
	if move.FaceDown {
		moveType = moveTypeFaceDown
	}
	newState, err := game.ApplyMove(state, playerIndex, move.Card, move.FaceDown)
	if err != nil {
		return game.GameState{}, recordedMove{}, err
	}
	return newState, recordedMove{
		PlayerIndex: playerIndex,
		Suit:        move.Card.Suit,
		Rank:        move.Card.Rank,
		Type:        moveType,
	}, nil
}

// resolveAceCloseMethod decides which end an Ace close targets. An explicit
// client method is used as-is (ApplyAceClose validates it). Otherwise the
// locked global method wins; if none is locked the method is inferred when
// exactly one end is legal, and is reported ambiguous when both are.
func resolveAceCloseMethod(state game.GameState, playerIndex int, suit game.Suit, requested string) (game.CloseMethod, error) {
	if requested != "" {
		return game.CloseMethod(requested), nil
	}
	if state.CloseMethod != "" {
		return state.CloseMethod, nil
	}
	var option *game.AceCloseOption
	for _, candidate := range game.AceCloseOptions(state, state.Hands[playerIndex]) {
		if candidate.Suit == suit {
			opt := candidate
			option = &opt
			break
		}
	}
	if option == nil {
		return "", fmt.Errorf("cannot close %s: no Ace close available", suit)
	}
	switch {
	case option.CanLow && !option.CanHigh:
		return game.CloseLow, nil
	case option.CanHigh && !option.CanLow:
		return game.CloseHigh, nil
	default:
		return "", fmt.Errorf("ambiguous close for %s: specify low or high", suit)
	}
}
