package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

type GameResult struct {
	RoomID       string             `json:"room_id"`
	StartedAt    time.Time          `json:"started_at"`
	FinishedAt   time.Time          `json:"finished_at"`
	Players      []GameResultPlayer `json:"players"`
	InitialHands [][]ReplayCard     `json:"initial_hands,omitempty"`
	Moves        []ReplayMove       `json:"moves,omitempty"`
}

// ReplayCard is a single card in a replay payload. Suit is the engine string
// (spades/hearts/diamonds/clubs); Rank is the engine int (2..14, Ace=14).
type ReplayCard struct {
	Suit string `json:"suit"`
	Rank int    `json:"rank"`
}

// ReplayMove is a single recorded move. Type is one of "play", "face_down", or
// "ace_close"; AceDirection ("low"/"high") is set only for ace_close moves.
type ReplayMove struct {
	Index        int    `json:"index"`
	PlayerIndex  int    `json:"player_index"`
	Suit         string `json:"suit"`
	Rank         int    `json:"rank"`
	Type         string `json:"type"`
	AceDirection string `json:"ace_direction,omitempty"`
}

// Replay is the full replay payload returned by GetReplay: game metadata, the
// initial dealt hands per seat, and the ordered move sequence.
type Replay struct {
	GameID       string         `json:"game_id"`
	RoomName     string         `json:"room_name"`
	StartedAt    string         `json:"started_at"`
	FinishedAt   string         `json:"finished_at"`
	Players      []ReplayPlayer `json:"players"`
	InitialHands [][]ReplayCard `json:"initial_hands"`
	Moves        []ReplayMove   `json:"moves"`
}

// ReplayPlayer labels a seat in the replay (index 0..3).
type ReplayPlayer struct {
	PlayerIndex int    `json:"player_index"`
	DisplayName string `json:"display_name"`
	IsBot       bool   `json:"is_bot"`
	IsWinner    bool   `json:"is_winner"`
	Rank        int    `json:"rank"`
}

// replayRetention is the number of most-recent games (by finished_at) whose
// replay data is kept. Older games' moves and initial hands are pruned on each
// save so storage stays bounded.
const replayRetention = 20

type GameResultPlayer struct {
	UserID        string `json:"user_id,omitempty"`
	DisplayName   string `json:"display_name"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
	IsBot         bool   `json:"is_bot"`
	// Index is the stable seat (0..3). Used as the game_players key so two
	// players sharing a display name don't collide.
	Index int `json:"index"`
}

type HistoryGame struct {
	GameID          string `json:"game_id"`
	RoomID          string `json:"room_id"`
	RoomName        string `json:"room_name"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	PenaltyPoints   int    `json:"penalty_points"`
	Rank            int    `json:"rank"`
	IsWinner        bool   `json:"is_winner"`
	RatingDelta     *int   `json:"rating_delta"`
	ReplayAvailable bool   `json:"replay_available"`
}

type PlayerDelta struct {
	UserID      string `json:"user_id"`
	RatingDelta int    `json:"rating_delta"`
	RatingAfter int    `json:"rating_after"`
	XPDelta     int    `json:"xp_delta"`
	XPAfter     int64  `json:"xp_after"`
	Level       int    `json:"level"`
}

type GameSaveResult struct {
	GameID uuid.UUID     `json:"game_id"`
	Deltas []PlayerDelta `json:"deltas"`
}

// closeGameMargins are penalty thresholds (inclusive) that classify a finish as
// "close" or a "blowout" relative to the winner / runner-up.
const (
	closeMargin   = 3
	blowoutMargin = 15
)

// gamePenalties captures the table-level penalty landmarks used to derive the
// per-player close-game story flags. A value of -1 means "unset" (no winner /
// no runner-up), distinguished from a legitimate winner penalty of 0.
type gamePenalties struct {
	winner int // best (lowest) penalty among winners; -1 if none
	second int // best penalty among non-winners; -1 if none
	worst  int // highest penalty at the table
}

// closeGameStoryFlags are the four close-game outcome flags for one player.
type closeGameStoryFlags struct {
	CloseWin    bool
	CloseLoss   bool
	BlowoutWin  bool
	BlowoutLoss bool
}

// closeGamePenalties scans every player (registered, guest, or bot) so the
// margins reflect the actual table outcome regardless of who is registered.
func closeGamePenalties(players []GameResultPlayer) gamePenalties {
	pen := gamePenalties{winner: -1, second: -1, worst: 0}
	for _, p := range players {
		if p.IsWinner && (pen.winner == -1 || p.PenaltyPoints < pen.winner) {
			pen.winner = p.PenaltyPoints
		}
		if p.PenaltyPoints > pen.worst {
			pen.worst = p.PenaltyPoints
		}
	}
	for _, p := range players {
		// The winning score itself is not a "runner-up"; everyone else is.
		if p.IsWinner && p.PenaltyPoints == pen.winner {
			continue
		}
		if pen.second == -1 || p.PenaltyPoints < pen.second {
			pen.second = p.PenaltyPoints
		}
	}
	return pen
}

// flagsFor derives a single player's close-game story flags from the table
// landmarks. The winner's margin is (runner-up − winner); a small margin is a
// close win, a large one a blowout win. Losers are measured by how far behind
// the winner they finished. A blowout loss additionally requires last place.
func (pen gamePenalties) flagsFor(p GameResultPlayer) closeGameStoryFlags {
	return closeGameStoryFlags{
		CloseWin:    p.IsWinner && pen.second != -1 && p.PenaltyPoints >= pen.second-closeMargin,
		CloseLoss:   !p.IsWinner && pen.winner != -1 && p.PenaltyPoints <= pen.winner+closeMargin,
		BlowoutWin:  p.IsWinner && pen.second != -1 && p.PenaltyPoints <= pen.second-blowoutMargin,
		BlowoutLoss: !p.IsWinner && pen.winner != -1 && p.PenaltyPoints >= pen.winner+blowoutMargin && p.PenaltyPoints >= pen.worst,
	}
}

// gameIDFor derives a deterministic game UUID from the room and finish time so
// that a retried/posted-twice result submission maps to the same primary key.
// finishedAt is truncated to the second because a room finishes exactly once and
// the WS service may resubmit after a transient API error.
func gameIDFor(roomID string, finishedAt time.Time) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(roomID+"|"+finishedAt.Truncate(time.Second).UTC().Format(time.RFC3339)))
}

// loadSavedGameDeltas returns the previously-saved player deltas for a game that
// was already recorded. Used by SaveGame to make submits idempotent.
func loadSavedGameDeltas(db *sql.DB, gameID uuid.UUID) (GameSaveResult, bool, error) {
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM games WHERE id = $1)`, gameID).Scan(&exists); err != nil {
		return GameSaveResult{}, false, fmt.Errorf("check existing game: %w", err)
	}
	if !exists {
		return GameSaveResult{}, false, nil
	}
	rows, err := db.Query(`
		SELECT ge.user_id, ge.rating_delta, ge.rating_after, xe.xp_delta, xe.xp_after
		FROM player_rating_events ge
		JOIN player_xp_events xe ON xe.game_id = ge.game_id AND xe.user_id = ge.user_id
		WHERE ge.game_id = $1
	`, gameID)
	if err != nil {
		return GameSaveResult{}, false, fmt.Errorf("load saved deltas: %w", err)
	}
	defer rows.Close()
	deltas := make([]PlayerDelta, 0)
	for rows.Next() {
		var uid string
		var d PlayerDelta
		if err := rows.Scan(&uid, &d.RatingDelta, &d.RatingAfter, &d.XPDelta, &d.XPAfter); err != nil {
			return GameSaveResult{}, false, fmt.Errorf("scan saved delta: %w", err)
		}
		d.UserID = uid
		d.Level = LevelFromXP(int64(d.XPAfter))
		deltas = append(deltas, d)
	}
	if err := rows.Err(); err != nil {
		return GameSaveResult{}, false, fmt.Errorf("iterate saved deltas: %w", err)
	}
	return GameSaveResult{GameID: gameID, Deltas: deltas}, true, nil
}

func SaveGame(db *sql.DB, result GameResult) (GameSaveResult, error) {
	// Resolve (and lazily roll over) the active season before opening the save
	// transaction, so the per-player season upsert and ELO update target the
	// right bucket. A failure here is non-fatal: the all-time stats path must
	// still record the game, so we fall back to an empty season id (skipped).
	var empty GameSaveResult
	seasonID := ""
	if season, err := EnsureActiveSeason(db); err != nil {
		log.Printf("ensure active season: %v", err)
	} else {
		seasonID = season.ID
	}

	// Idempotency: a room finishes exactly once, so derive a stable game id and
	// short-circuit to the previously-saved deltas if we already recorded it
	// (e.g. the WS service retried after a network hiccup). This avoids double
	// counting stats, XP, rating, and achievements.
	gameID := gameIDFor(result.RoomID, result.FinishedAt)
	if existing, ok, err := loadSavedGameDeltas(db, gameID); err != nil {
		return empty, err
	} else if ok {
		return existing, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return empty, fmt.Errorf("begin save game: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(); err != nil {
				log.Printf("rollback save game: %v", err)
			}
		}
	}()

	if _, err := tx.Exec(`INSERT INTO games (id, room_id, room_name, started_at, finished_at) VALUES ($1, $2, (SELECT name FROM rooms WHERE id::text = $2), $3, $4) ON CONFLICT (id) DO NOTHING`, gameID, result.RoomID, result.StartedAt, result.FinishedAt); err != nil {
		return empty, fmt.Errorf("insert game: %w", err)
	}

	registeredWinners := 0
	hasBot := false
	for _, player := range result.Players {
		if player.IsWinner && player.UserID != "" {
			registeredWinners++
		}
		if player.IsBot {
			hasBot = true
		}
	}
	sharedWinCount := registeredWinners

	allZeroPenalty := true
	aceClosed := false
	for _, player := range result.Players {
		if !player.IsBot && player.PenaltyPoints > 0 {
			allZeroPenalty = false
		}
	}
	for _, move := range result.Moves {
		if move.Type == "ace_close" {
			aceClosed = true
			break
		}
	}
	gameDurationSeconds := int(result.FinishedAt.Sub(result.StartedAt).Seconds())
	if gameDurationSeconds < 0 {
		gameDurationSeconds = 0
	}

	pen := closeGamePenalties(result.Players)

	eloPlayers := []EloPlayer{}
	xpSnapshots := map[string]struct {
		xpDelta int
		xpAfter int64
	}{}
	for _, player := range result.Players {
		var userID *uuid.UUID
		if player.UserID != "" {
			parsed, err := uuid.Parse(player.UserID)
			if err != nil {
				return empty, fmt.Errorf("parse player user id: %w", err)
			}
			userID = &parsed
		}
		res, err := tx.Exec(`
			INSERT INTO game_players (game_id, player_index, user_id, display_name, penalty_points, rank, is_winner, is_bot)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (game_id, player_index) DO NOTHING
		`, gameID, player.Index, userID, player.DisplayName, player.PenaltyPoints, player.Rank, player.IsWinner, player.IsBot)
		if err != nil {
			return empty, fmt.Errorf("insert game player: %w", err)
		}
		playerAlreadySaved := false
		if n, err := res.RowsAffected(); err == nil && n == 0 {
			playerAlreadySaved = true
		}
		if userID != nil && !playerAlreadySaved {
			flags := pen.flagsFor(player)

			xpDelta, xpBreakdown := CalculateXP(player, hasBot)

			params := UpsertUserStatsParams{
				UserID:      *userID,
				IsWinner:    player.IsWinner,
				Penalty:     player.PenaltyPoints,
				Rank:        player.Rank,
				HasBot:      hasBot,
				CloseWin:    flags.CloseWin,
				CloseLoss:   flags.CloseLoss,
				BlowoutWin:  flags.BlowoutWin,
				BlowoutLoss: flags.BlowoutLoss,
				XPDelta:     xpDelta,
			}
			snap, err := UpsertUserStats(tx, params)
			if err != nil {
				return empty, err
			}
			if err := insertXPEvent(tx, gameID, *userID, snap.XP, xpDelta, xpBreakdown); err != nil {
				return empty, err
			}
			xpSnapshots[userID.String()] = struct {
				xpDelta int
				xpAfter int64
			}{xpDelta, snap.XP}
			if seasonID != "" {
				if err := UpsertSeasonUserStats(tx, seasonID, params); err != nil {
					return empty, err
				}
			}
			eloPlayers = append(eloPlayers, EloPlayer{UserID: *userID, Rank: player.Rank})
			sWinCount := 0
			if player.IsWinner {
				sWinCount = sharedWinCount
			}
			ids, err := EvaluateAchievementIDs(tx, achievementContext{
				IsWinner:            player.IsWinner,
				SharedWinCount:      sWinCount,
				Penalty:             player.PenaltyPoints,
				GamesPlayed:         snap.GamesPlayed,
				Wins:                snap.Wins,
				CurrentStreak:       snap.CurrentStreak,
				CurrentTop2Streak:   snap.CurrentTop2Streak,
				FirstPlaceCount:     snap.FirstPlaceCount,
				ZeroPenaltyGames:    snap.ZeroPenaltyGames,
				HumanOnlyGames:      snap.HumanOnlyGames,
				AllZeroPenalty:      allZeroPenalty,
				AceClosed:           aceClosed,
				GameDurationSeconds: gameDurationSeconds,
			})
			if err != nil {
				return empty, err
			}
			if err := AwardAchievements(tx, *userID, ids); err != nil {
				return empty, err
			}
		}
	}

	eloDeltas := map[string]int{}
	if len(eloPlayers) >= 2 {
		eloDeltas, err = applyEloUpdates(tx, seasonID, eloPlayers)
		if err != nil {
			return empty, err
		}
	}

	for _, ep := range eloPlayers {
		delta := eloDeltas[ep.UserID.String()]
		if err := insertRatingEvent(tx, gameID, ep.UserID, delta); err != nil {
			return empty, err
		}
	}

	if err := insertReplay(tx, gameID, result.InitialHands, result.Moves); err != nil {
		return empty, err
	}
	if err := pruneOldReplays(tx); err != nil {
		return empty, err
	}

	if err := tx.Commit(); err != nil {
		return empty, fmt.Errorf("commit save game: %w", err)
	}
	committed = true

	deltas := make([]PlayerDelta, 0, len(eloPlayers))
	for _, ep := range eloPlayers {
		uid := ep.UserID.String()
		xpSnap := xpSnapshots[uid]
		ratingDelta := eloDeltas[uid]
		var ratingAfter int
		if err := db.QueryRow(`SELECT rating FROM user_stats WHERE user_id = $1`, ep.UserID).Scan(&ratingAfter); err != nil {
			ratingAfter = 1200 + ratingDelta
		}
		deltas = append(deltas, PlayerDelta{
			UserID:      uid,
			RatingDelta: ratingDelta,
			RatingAfter: ratingAfter,
			XPDelta:     xpSnap.xpDelta,
			XPAfter:     xpSnap.xpAfter,
			Level:       LevelFromXP(xpSnap.xpAfter),
		})
	}

	return GameSaveResult{GameID: gameID, Deltas: deltas}, nil
}

// applyEloUpdates reads the current lifetime (and season) ratings for the given
// registered players, computes pairwise ELO deltas from their ranks, and writes
// the new ratings back inside the save transaction. Lifetime and season ratings
// move independently from their own baselines. Returns the *lifetime* deltas per
// user for the rating-events log, which is anchored to user_stats.rating; the
// season delta is applied to season_user_stats and intentionally excluded here.
func applyEloUpdates(tx *sql.Tx, seasonID string, players []EloPlayer) (map[string]int, error) {
	ids := make([]uuid.UUID, len(players))
	for i, p := range players {
		ids[i] = p.UserID
	}

	// Lifetime ratings.
	lifetime, err := ReadRatings(tx, ids)
	if err != nil {
		return nil, err
	}
	lifePlayers := make([]EloPlayer, len(players))
	for i, p := range players {
		lifePlayers[i] = EloPlayer{UserID: p.UserID, Rank: p.Rank, Rating: lifetime[p.UserID]}
	}
	lifeDeltas := ComputeEloDeltas(lifePlayers)

	// Season ratings (independent baseline).
	seasonDeltas := map[uuid.UUID]int{}
	if seasonID != "" {
		seasonRatings, err := ReadSeasonRatings(tx, seasonID, ids)
		if err != nil {
			return nil, err
		}
		seasonPlayers := make([]EloPlayer, len(players))
		for i, p := range players {
			seasonPlayers[i] = EloPlayer{UserID: p.UserID, Rank: p.Rank, Rating: seasonRatings[p.UserID]}
		}
		seasonDeltas = ComputeEloDeltas(seasonPlayers)
	}

	lifetimeOut := make(map[string]int, len(players))
	for id, delta := range lifeDeltas {
		if err := ApplyRatingDelta(tx, id, delta); err != nil {
			return nil, err
		}
		lifetimeOut[id.String()] = delta
	}
	for id, delta := range seasonDeltas {
		if err := ApplySeasonRatingDelta(tx, seasonID, id, delta); err != nil {
			return nil, err
		}
	}
	// Fill in zeros for players with no lifetime movement.
	for _, p := range players {
		if _, ok := lifetimeOut[p.UserID.String()]; !ok {
			lifetimeOut[p.UserID.String()] = 0
		}
	}

	return lifetimeOut, nil
}

// insertRatingEvent records the per-game rating change for a registered player.
// It reads the rating AFTER the delta was applied (the current value in
// user_stats) and writes the before/after/delta row.
func insertRatingEvent(tx *sql.Tx, gameID, userID uuid.UUID, delta int) error {
	var ratingAfter int
	if err := tx.QueryRow(`SELECT rating FROM user_stats WHERE user_id = $1`, userID).Scan(&ratingAfter); err != nil {
		return fmt.Errorf("read rating for event: %w", err)
	}
	ratingBefore := ratingAfter - delta
	if _, err := tx.Exec(`
		INSERT INTO player_rating_events (game_id, user_id, rating_before, rating_after, rating_delta)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (game_id, user_id) DO NOTHING
	`, gameID, userID, ratingBefore, ratingAfter, delta); err != nil {
		return fmt.Errorf("insert rating event: %w", err)
	}
	return nil
}

// insertXPEvent records the per-game XP award for a registered player. xpAfter
// is the lifetime total returned by UpsertUserStats, so xpBefore is derived as
// xpAfter - delta. The breakdown is stored as JSONB for later explanation.
func insertXPEvent(tx *sql.Tx, gameID, userID uuid.UUID, xpAfter int64, delta int, breakdown XPBreakdown) error {
	payload, err := json.Marshal(breakdown)
	if err != nil {
		return fmt.Errorf("marshal xp breakdown: %w", err)
	}
	xpBefore := xpAfter - int64(delta)
	if _, err := tx.Exec(`
		INSERT INTO player_xp_events (game_id, user_id, xp_before, xp_after, xp_delta, breakdown)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id, user_id) DO NOTHING
	`, gameID, userID, xpBefore, xpAfter, delta, payload); err != nil {
		return fmt.Errorf("insert xp event: %w", err)
	}
	return nil
}

func GetPlayerHistory(db *sql.DB, userID uuid.UUID, page int, perPage int) ([]HistoryGame, int, error) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM game_players WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count history: %w", err)
	}

	rows, err := db.Query(`
		SELECT g.id, g.room_id, g.room_name, g.started_at, g.finished_at,
		       gp.penalty_points, gp.rank, gp.is_winner,
		       pre.rating_delta,
		       EXISTS(SELECT 1 FROM game_initial_hands h WHERE h.game_id = g.id) AS replay_available
		FROM game_players gp
		JOIN games g ON g.id = gp.game_id
		LEFT JOIN player_rating_events pre ON pre.game_id = g.id AND pre.user_id = gp.user_id
		WHERE gp.user_id = $1
		ORDER BY g.finished_at DESC
		LIMIT $2 OFFSET $3
	`, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	games := []HistoryGame{}
	for rows.Next() {
		var game HistoryGame
		var startedAt, finishedAt time.Time
		var roomName sql.NullString
		var ratingDelta sql.NullInt32
		if err := rows.Scan(&game.GameID, &game.RoomID, &roomName, &startedAt, &finishedAt, &game.PenaltyPoints, &game.Rank, &game.IsWinner, &ratingDelta, &game.ReplayAvailable); err != nil {
			return nil, 0, fmt.Errorf("scan history: %w", err)
		}
		game.RoomName = roomName.String
		game.StartedAt = startedAt.UTC().Format(time.RFC3339)
		game.FinishedAt = finishedAt.UTC().Format(time.RFC3339)
		if ratingDelta.Valid {
			v := int(ratingDelta.Int32)
			game.RatingDelta = &v
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate history: %w", err)
	}
	return games, total, nil
}

type RatingEvent struct {
	GameID       string `json:"game_id"`
	RatingBefore int    `json:"rating_before"`
	RatingAfter  int    `json:"rating_after"`
	RatingDelta  int    `json:"rating_delta"`
	CreatedAt    string `json:"created_at"`
}

func GetRatingHistory(db *sql.DB, userID uuid.UUID, page, perPage int) ([]RatingEvent, int, error) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM player_rating_events WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rating events: %w", err)
	}

	rows, err := db.Query(`
		SELECT game_id, rating_before, rating_after, rating_delta, created_at
		FROM player_rating_events
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query rating events: %w", err)
	}
	defer rows.Close()

	events := []RatingEvent{}
	for rows.Next() {
		var e RatingEvent
		var createdAt time.Time
		if err := rows.Scan(&e.GameID, &e.RatingBefore, &e.RatingAfter, &e.RatingDelta, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan rating event: %w", err)
		}
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rating events: %w", err)
	}
	return events, total, nil
}

// insertReplay persists the initial dealt hands and the ordered move log for a
// game inside the save transaction. A game with no moves (e.g. practice mode is
// not saved at all, but defensively) writes nothing.
func insertReplay(tx *sql.Tx, gameID uuid.UUID, initialHands [][]ReplayCard, moves []ReplayMove) error {
	if len(moves) == 0 {
		return nil
	}
	for seat, hand := range initialHands {
		payload, err := json.Marshal(hand)
		if err != nil {
			return fmt.Errorf("marshal initial hand: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO game_initial_hands (game_id, player_index, hand)
			VALUES ($1, $2, $3)
			ON CONFLICT (game_id, player_index) DO NOTHING
		`, gameID, seat, payload); err != nil {
			return fmt.Errorf("insert initial hand: %w", err)
		}
	}
	for i, m := range moves {
		idx := m.Index
		if idx == 0 && i != 0 {
			idx = i
		}
		var aceDir interface{}
		if m.AceDirection != "" {
			aceDir = m.AceDirection
		}
		if _, err := tx.Exec(`
			INSERT INTO game_moves (game_id, move_index, player_index, card_rank, card_suit, move_type, ace_close_direction)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (game_id, move_index) DO NOTHING
		`, gameID, idx, m.PlayerIndex, m.Rank, suitToCode(m.Suit), m.Type, aceDir); err != nil {
			return fmt.Errorf("insert move: %w", err)
		}
	}
	return nil
}

// pruneOldReplays deletes replay data (moves + initial hands) for games outside
// the most-recent replayRetention window, keeping storage bounded.
func pruneOldReplays(tx *sql.Tx) error {
	const keep = `
		SELECT id FROM games ORDER BY finished_at DESC LIMIT $1
	`
	if _, err := tx.Exec(`DELETE FROM game_moves WHERE game_id NOT IN (`+keep+`)`, replayRetention); err != nil {
		return fmt.Errorf("prune game moves: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM game_initial_hands WHERE game_id NOT IN (`+keep+`)`, replayRetention); err != nil {
		return fmt.Errorf("prune initial hands: %w", err)
	}
	return nil
}

// suit codes map the engine suit string to the SMALLINT stored in game_moves.
var suitCodes = map[string]int{"spades": 0, "hearts": 1, "diamonds": 2, "clubs": 3}
var suitNames = map[int]string{0: "spades", 1: "hearts", 2: "diamonds", 3: "clubs"}

func suitToCode(suit string) int {
	if code, ok := suitCodes[suit]; ok {
		return code
	}
	return 0
}

func suitFromCode(code int) string {
	if name, ok := suitNames[code]; ok {
		return name
	}
	return "spades"
}

// GetReplay returns the full replay for a game: metadata, the initial dealt
// hands per seat, and the ordered move sequence. ok=false when no replay data
// exists (the game is older than the retention window, or never recorded).
func GetReplay(db *sql.DB, gameID uuid.UUID) (Replay, bool, error) {
	var replay Replay
	var roomName sql.NullString
	var startedAt, finishedAt time.Time
	err := db.QueryRow(`
		SELECT id, room_name, started_at, finished_at FROM games WHERE id = $1
	`, gameID).Scan(&replay.GameID, &roomName, &startedAt, &finishedAt)
	if err == sql.ErrNoRows {
		return Replay{}, false, nil
	}
	if err != nil {
		return Replay{}, false, fmt.Errorf("query game: %w", err)
	}
	replay.RoomName = roomName.String
	replay.StartedAt = startedAt.UTC().Format(time.RFC3339)
	replay.FinishedAt = finishedAt.UTC().Format(time.RFC3339)

	playerRows, err := db.Query(`
		SELECT display_name, is_bot, is_winner, rank, player_index
		FROM game_players WHERE game_id = $1 ORDER BY player_index ASC
	`, gameID)
	if err != nil {
		return Replay{}, false, fmt.Errorf("query replay players: %w", err)
	}
	defer playerRows.Close()
	for playerRows.Next() {
		var p ReplayPlayer
		if err := playerRows.Scan(&p.DisplayName, &p.IsBot, &p.IsWinner, &p.Rank, &p.PlayerIndex); err != nil {
			return Replay{}, false, fmt.Errorf("scan replay player: %w", err)
		}
		replay.Players = append(replay.Players, p)
	}
	if err := playerRows.Err(); err != nil {
		return Replay{}, false, fmt.Errorf("iterate replay players: %w", err)
	}

	handRows, err := db.Query(`
		SELECT player_index, hand FROM game_initial_hands
		WHERE game_id = $1 ORDER BY player_index ASC
	`, gameID)
	if err != nil {
		return Replay{}, false, fmt.Errorf("query initial hands: %w", err)
	}
	defer handRows.Close()
	hands := [][]ReplayCard{}
	hasReplay := false
	for handRows.Next() {
		var idx int
		var payload []byte
		if err := handRows.Scan(&idx, &payload); err != nil {
			return Replay{}, false, fmt.Errorf("scan initial hand: %w", err)
		}
		var hand []ReplayCard
		if err := json.Unmarshal(payload, &hand); err != nil {
			return Replay{}, false, fmt.Errorf("unmarshal initial hand: %w", err)
		}
		hands = append(hands, hand)
		hasReplay = true
	}
	if err := handRows.Err(); err != nil {
		return Replay{}, false, fmt.Errorf("iterate initial hands: %w", err)
	}
	if !hasReplay {
		return Replay{}, false, nil
	}
	replay.InitialHands = hands

	moveRows, err := db.Query(`
		SELECT move_index, player_index, card_rank, card_suit, move_type, ace_close_direction
		FROM game_moves WHERE game_id = $1 ORDER BY move_index ASC
	`, gameID)
	if err != nil {
		return Replay{}, false, fmt.Errorf("query moves: %w", err)
	}
	defer moveRows.Close()
	moves := []ReplayMove{}
	for moveRows.Next() {
		var m ReplayMove
		var rank, suit sql.NullInt32
		var aceDir sql.NullString
		if err := moveRows.Scan(&m.Index, &m.PlayerIndex, &rank, &suit, &m.Type, &aceDir); err != nil {
			return Replay{}, false, fmt.Errorf("scan move: %w", err)
		}
		m.Rank = int(rank.Int32)
		m.Suit = suitFromCode(int(suit.Int32))
		m.AceDirection = aceDir.String
		moves = append(moves, m)
	}
	if err := moveRows.Err(); err != nil {
		return Replay{}, false, fmt.Errorf("iterate moves: %w", err)
	}
	replay.Moves = moves

	return replay, true, nil
}
