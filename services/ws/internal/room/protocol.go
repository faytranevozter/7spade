package room

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

var orderedSuits = []game.Suit{game.Spades, game.Hearts, game.Diamonds, game.Clubs}

const (
	messageTypeError              = "error"
	messageTypeGameOver           = "game_over"
	messageTypePlaceFaceDown      = "place_facedown"
	messageTypePlayerDisconnected = "player_disconnected"
	messageTypePlayerReconnected  = "player_reconnected"
	messageTypeRematchCancelled   = "rematch_cancelled"
	messageTypeRematchStatus      = "rematch_status"
	messageTypeRematchCountdown   = "rematch_countdown"
	messageTypeRematchVote        = "rematch_vote"
	messageTypeGoToWaitingRoom    = "go_to_waiting_room"
	messageTypeRoomClosed         = "room_closed"
	messageTypePlayCard           = "play_card"
	messageTypeStateUpdate        = "state_update"
	messageTypeSpectatorState     = "spectator_state"
	messageTypeEmote              = "emote"
	messageTypeSpectatorEmote     = "spectator_emote"
)

// spectatorRole is the query-param value that opens a read-only spectator
// connection (ws://host/ws?room_id=X&token=JWT&role=spectator) instead of
// taking a seat.
const spectatorRole = "spectator"

// allowedEmotes is the server-side allowlist of emote IDs. Emotes outside this
// set are rejected so the broadcast channel can't be abused to relay arbitrary
// payloads. The frontend catalog (web/src/game/emotes.ts) must stay in sync.
var allowedEmotes = map[string]bool{
	"thumbs_up": true,
	"laugh":     true,
	"wow":       true,
	"think":     true,
	"celebrate": true,
	"sad":       true,
	"gg":        true,
	"nice":      true,
	"oops":      true,
}

// emoteCooldown is the minimum gap between emotes from a single player. Faster
// emotes are silently dropped to prevent spamming the room.
const emoteCooldown = time.Second

// spectatorEmoteCooldown is the minimum gap between emotes from a single
// spectator. Spectators are an open, potentially large audience, so they get a
// stricter limit than seated players; faster emotes are silently dropped.
const spectatorEmoteCooldown = 2 * time.Second

// spectatorIDCounter assigns each spectator connection a process-unique id used
// to attribute emote broadcasts. The id only needs to disambiguate concurrent
// spectators of a room (so two anonymous/guest viewers don't collide); it is
// never persisted and is paired with the replica id to stay unique across the
// cluster.
var spectatorIDCounter atomic.Uint64

func errorMessage(message string) map[string]any {
	return map[string]any{"type": messageTypeError, "message": message}
}

// fatalErrorMessage marks an error that ends the connection (a rejected join),
// so the client can route the user away with the reason rather than treating it
// as a transient in-game error toast.
func fatalErrorMessage(message string) map[string]any {
	return map[string]any{"type": messageTypeError, "message": message, "fatal": true}
}

func cardPayload(card game.Card, valid bool) map[string]any {
	return map[string]any{"suit": card.Suit, "rank": rankString(card.Rank), "valid": valid}
}

func boardPayload(state game.GameState) map[string]any {
	board := map[string]any{}
	for _, suit := range orderedSuits {
		sequence, ok := state.Board[suit]
		if !ok {
			board[string(suit)] = nil
			continue
		}
		payload := map[string]any{"low": sequence.Low, "high": sequence.High}
		if state.Config.DeckCount > 1 && len(sequence.Stacks) > 0 {
			stacks := map[string]int{}
			for rank, count := range sequence.Stacks {
				if count > 1 {
					stacks[rankString(rank)] = count
				}
			}
			if len(stacks) > 0 {
				payload["stacks"] = stacks
			}
		}
		board[string(suit)] = payload
	}
	return board
}

func closedSuits(state game.GameState) []string {
	closed := []string{}
	for _, suit := range orderedSuits {
		if state.Closed[suit] {
			closed = append(closed, string(suit))
		}
	}
	return closed
}

func parseCard(suitValue string, rankValue string) (game.Card, error) {
	suit := game.Suit(strings.ToLower(suitValue))
	if suit != game.Spades && suit != game.Hearts && suit != game.Diamonds && suit != game.Clubs {
		return game.Card{}, fmt.Errorf("unknown suit: %s", suitValue)
	}
	rank, err := parseRank(rankValue)
	if err != nil {
		return game.Card{}, err
	}
	return game.Card{Suit: suit, Rank: rank}, nil
}

func parseRank(value string) (game.Rank, error) {
	switch strings.ToUpper(value) {
	case "J":
		return game.Jack, nil
	case "Q":
		return game.Queen, nil
	case "K":
		return game.King, nil
	case "A":
		return game.Ace, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < int(game.Two) || number > int(game.Ace) {
		return 0, fmt.Errorf("unknown rank: %s", value)
	}
	return game.Rank(number), nil
}

func rankString(rank game.Rank) string {
	switch rank {
	case game.Jack:
		return "J"
	case game.Queen:
		return "Q"
	case game.King:
		return "K"
	case game.Ace:
		return "A"
	default:
		return strconv.Itoa(int(rank))
	}
}
