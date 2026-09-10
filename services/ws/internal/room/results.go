package room

import (
	"log"
	"sort"
	"time"

	"github.com/faytranevozter/7spade/services/ws/game"
)

type gameHistoryStore interface {
	SaveGame(result savedGameResult) (string, []playerDelta, error)
}

// recordedMove captures a single applied move for replay. PlayerIndex is the
// seat index 0..PlayerCount-1, Suit/Rank identify the card, Type is one of "play", "face_down",
// or "ace_close", and AceDirection is "low" or "high" only for ace_close moves.
type recordedMove struct {
	PlayerIndex  int
	Suit         game.Suit
	Rank         game.Rank
	Type         string
	AceDirection game.CloseMethod
}

// Move type constants used by the replay system to describe how a card was
// played. Match the values stored in the API's game_moves.move_type column.
const (
	moveTypePlay     = "play"
	moveTypeFaceDown = "face_down"
	moveTypeAceClose = "ace_close"
)

func toSavedCards(cards []game.Card) []savedCard {
	out := make([]savedCard, len(cards))
	for i, c := range cards {
		out[i] = savedCard{Suit: string(c.Suit), Rank: int(c.Rank)}
	}
	return out
}

func toSavedMoves(moves []recordedMove) []savedReplayMove {
	out := make([]savedReplayMove, len(moves))
	for i, m := range moves {
		out[i] = savedReplayMove{
			Index:        i,
			PlayerIndex:  m.PlayerIndex,
			Suit:         string(m.Suit),
			Rank:         int(m.Rank),
			Type:         m.Type,
			AceDirection: string(m.AceDirection),
		}
	}
	return out
}

// gameOverMessage builds the game_over payload for a single recipient (e.g. a
// player reconnecting to an already-finished room).
func (room *room) gameOverMessage() map[string]any {
	room.mu.Lock()
	defer room.mu.Unlock()
	return room.gameOverMessageLocked("")
}

func (room *room) gameOverMessageFor(userID string) map[string]any {
	room.mu.Lock()
	defer room.mu.Unlock()
	return room.gameOverMessageLocked(userID)
}

// gameOverMessageLocked builds the game_over payload, including the final board
// so reconnecting clients (with no prior state_update this session) can still
// render the completed board alongside the results. Caller must hold room.mu.
func (room *room) gameOverMessageLocked(recipientID string) map[string]any {
	message := map[string]any{
		"type":             messageTypeGameOver,
		"results":          room.results(),
		"board":            boardPayload(room.state),
		"closed_suits":     closedSuits(room.state),
		"ace_close_method": room.state.CloseMethod,
		"practice_mode":    room.practiceMode,
		"team_mode":        string(room.gameConfig.TeamMode),
		"spectator_count":  len(room.spectators),
		"game_id":          room.savedGameID,
	}
	if delta, ok := room.gameDeltas[recipientID]; ok && len(delta.NewSkinGrants) > 0 {
		message["new_skin_grants"] = delta.NewSkinGrants
	}
	return message
}

func (room *room) saveGameResult() {
	if room == nil {
		return
	}
	// Persisting the result is authoritative: only the owner does it, so a
	// failed-over replica can't double-save the same finished game.
	if !room.isOwnerOrSolo() {
		return
	}
	room.mu.Lock()
	result := room.savedResultLocked(time.Now().UTC())
	historyStore := room.gameHistory
	statusUpdater := room.statusUpdater
	practiceMode := room.practiceMode
	roomID := room.id
	room.mu.Unlock()
	// Practice games are solo vs bots: never recorded to history or stats, so a
	// practice round can't pollute the leaderboard. Status is still flipped to
	// 'finished' so the room can be reconciled/cleaned up like any other.
	if historyStore != nil && !practiceMode {
		gameID, deltas, err := historyStore.SaveGame(result)
		if err != nil {
			log.Printf("save game result: %v", err)
		} else {
			room.mu.Lock()
			room.savedGameID = gameID
			if len(deltas) > 0 {
				deltaMap := make(map[string]playerDelta, len(deltas))
				for _, d := range deltas {
					deltaMap[d.UserID] = d
				}
				room.gameDeltas = deltaMap
			}
			// Persist after save so a WS restart still ships game_id / deltas
			// on the reconnect game_over payload.
			room.persistLocked()
			room.mu.Unlock()
		}
	}
	if statusUpdater != nil {
		if err := statusUpdater.UpdateRoomStatus(roomID, "finished"); err != nil {
			log.Printf("update room status to finished: %v", err)
		}
	}
}

func (room *room) savedResultLocked(finishedAt time.Time) savedGameResult {
	scoredPlayers := room.scoredPlayersLocked()
	players := make([]savedGamePlayer, 0, len(room.players))
	for _, scoredPlayer := range scoredPlayers {
		player := scoredPlayer.player
		userID := player.sub
		if player.isGuest || player.isBot {
			userID = ""
		}
		var team *int
		if room.gameConfig.TeamMode == game.Team2v2 {
			v := player.team
			team = &v
		}
		players = append(players, savedGamePlayer{
			UserID:        userID,
			SubjectID:     player.sub,
			DisplayName:   player.displayName,
			PenaltyPoints: scoredPlayer.score,
			Rank:          scoredPlayer.rank,
			IsWinner:      scoredPlayer.isWinner,
			IsBot:         player.isBot,
			IsGuest:       player.isGuest,
			Team:          team,
			FaceDownCards: savedRevealedFaceDownCards(room.state, player.index),
			Index:         player.index,
		})
	}
	startedAt := room.startedAt
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	var initialHands [][]savedCard
	if len(room.moves) > 0 {
		initialHands = make([][]savedCard, len(room.initialHands))
		for i := range room.initialHands {
			initialHands[i] = toSavedCards(room.initialHands[i])
		}
	}
	return savedGameResult{
		RoomID:       room.id,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		Players:      players,
		InitialHands: initialHands,
		Moves:        toSavedMoves(room.moves),
	}
}

func (room *room) results() []map[string]any {
	scoredPlayers := room.scoredPlayersLocked()
	results := make([]map[string]any, 0, len(scoredPlayers))
	for _, scoredPlayer := range scoredPlayers {
		player := scoredPlayer.player
		entry := map[string]any{
			"display_name":   player.displayName,
			"player_index":   player.index,
			"avatar_url":     player.avatar,
			"is_bot":         player.isBot,
			"facedown_cards": revealedFaceDownCards(room.state, player.index),
			"penalty_points": scoredPlayer.score,
			"rank":           scoredPlayer.rank,
			"is_winner":      scoredPlayer.isWinner,
		}
		if room.gameConfig.TeamMode == game.Team2v2 {
			entry["team"] = player.team
		}
		if !player.isBot && !player.isGuest {
			if d, ok := room.gameDeltas[player.sub]; ok {
				if d.RatingDelta != nil && d.RatingAfter != nil {
					entry["rating_delta"] = *d.RatingDelta
					entry["rating_after"] = *d.RatingAfter
				}
				entry["xp_delta"] = d.XPDelta
				entry["xp_after"] = d.XPAfter
				entry["level"] = d.Level
			}
		}
		results = append(results, entry)
	}
	return results
}

type scoredPlayer struct {
	player   *player
	score    int
	rank     int
	isWinner bool
}

func (room *room) scoredPlayersLocked() []scoredPlayer {
	scores := game.CalculateScores(room.state)
	sortedScores := append([]int(nil), scores...)
	sort.Ints(sortedScores)
	ranksByScore := competitionRanks(sortedScores)
	lowest := sortedScores[0]
	scoredPlayers := make([]scoredPlayer, 0, len(room.players))
	for _, player := range room.players {
		score := scores[player.index]
		scoredPlayers = append(scoredPlayers, scoredPlayer{player: player, score: score, rank: ranksByScore[score], isWinner: score == lowest})
	}
	return scoredPlayers
}

func competitionRanks(sortedScores []int) map[int]int {
	ranks := make(map[int]int, len(sortedScores))
	for index, score := range sortedScores {
		if _, exists := ranks[score]; exists {
			continue
		}
		ranks[score] = index + 1
	}
	return ranks
}

func revealedFaceDownCards(state game.GameState, playerIndex int) []map[string]any {
	cards := make([]map[string]any, 0, len(state.FaceDown[playerIndex]))
	for _, card := range state.FaceDown[playerIndex] {
		cards = append(cards, map[string]any{
			"suit":   string(card.Suit),
			"rank":   rankString(card.Rank),
			"points": game.ScoreCard(card, state),
		})
	}
	return cards
}

func savedRevealedFaceDownCards(state game.GameState, playerIndex int) []savedRevealedCard {
	cards := make([]savedRevealedCard, 0, len(state.FaceDown[playerIndex]))
	for _, card := range state.FaceDown[playerIndex] {
		cards = append(cards, savedRevealedCard{
			Suit:   string(card.Suit),
			Rank:   int(card.Rank),
			Points: game.ScoreCard(card, state),
		})
	}
	return cards
}
