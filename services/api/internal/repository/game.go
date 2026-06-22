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
	RoomID     string             `json:"room_id"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	Players    []GameResultPlayer `json:"players"`
}

type GameResultPlayer struct {
	UserID        string `json:"user_id,omitempty"`
	DisplayName   string `json:"display_name"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
	IsBot         bool   `json:"is_bot"`
}

type HistoryGame struct {
	GameID        string `json:"game_id"`
	RoomID        string `json:"room_id"`
	RoomName      string `json:"room_name"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	PenaltyPoints int    `json:"penalty_points"`
	Rank          int    `json:"rank"`
	IsWinner      bool   `json:"is_winner"`
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

func SaveGame(db *sql.DB, result GameResult) (uuid.UUID, error) {
	// Resolve (and lazily roll over) the active season before opening the save
	// transaction, so the per-player season upsert and ELO update target the
	// right bucket. A failure here is non-fatal: the all-time stats path must
	// still record the game, so we fall back to an empty season id (skipped).
	seasonID := ""
	if season, err := EnsureActiveSeason(db); err != nil {
		log.Printf("ensure active season: %v", err)
	} else {
		seasonID = season.ID
	}

	tx, err := db.Begin()
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin save game: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(); err != nil {
				log.Printf("rollback save game: %v", err)
			}
		}
	}()

	gameID := uuid.New()
	// Copy the room's current name into the game row so history keeps a friendly
	// label even after the rooms row is deleted. Match on id::text to avoid
	// casting room_id (which may be a non-UUID test value) to uuid; a missing
	// room yields NULL.
	if _, err := tx.Exec(`INSERT INTO games (id, room_id, room_name, started_at, finished_at) VALUES ($1, $2, (SELECT name FROM rooms WHERE id::text = $2), $3, $4)`, gameID, result.RoomID, result.StartedAt, result.FinishedAt); err != nil {
		return uuid.Nil, fmt.Errorf("insert game: %w", err)
	}

	// Determine game-level context before the per-player loop.
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
	sharedWin := registeredWinners > 1

	// closeGamePenalties is computed over *all* players (including bots and
	// guests), since the close-game / blowout margins describe the table
	// outcome regardless of who is registered. A bot or guest can finish first,
	// so anchoring these to registered players only would mis-stamp the flags.
	pen := closeGamePenalties(result.Players)

	// Collect registered players for the ELO pairwise calculation as we go.
	eloPlayers := []EloPlayer{}
	for _, player := range result.Players {
		var userID *uuid.UUID
		if player.UserID != "" {
			parsed, err := uuid.Parse(player.UserID)
			if err != nil {
				return uuid.Nil, fmt.Errorf("parse player user id: %w", err)
			}
			userID = &parsed
		}
		_, err := tx.Exec(`
			INSERT INTO game_players (game_id, user_id, display_name, penalty_points, rank, is_winner, is_bot)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, gameID, userID, player.DisplayName, player.PenaltyPoints, player.Rank, player.IsWinner, player.IsBot)
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert game player: %w", err)
		}
		// Update lifetime stats for registered players only; guests/bots have a
		// nil user_id and are skipped. Runs in the same transaction so stats
		// never diverge from the underlying game_players rows on the happy path.
		if userID != nil {
			flags := pen.flagsFor(player)

			// XP is awarded to registered players only and is folded into the
			// stats upsert below so user_stats is touched once. The breakdown is
			// recorded as an audit row after the upsert returns the new total.
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
				return uuid.Nil, err
			}
			// Audit the XP award. snap.XP is the post-update total, so
			// xp_before = xp_after - delta stays consistent within the txn.
			if err := insertXPEvent(tx, gameID, *userID, snap.XP, xpDelta, xpBreakdown); err != nil {
				return uuid.Nil, err
			}
			// Mirror into the active season's bucket (Phase A). XP is
			// lifetime-only, so UpsertSeasonUserStats ignores params.XPDelta.
			if seasonID != "" {
				if err := UpsertSeasonUserStats(tx, seasonID, params); err != nil {
					return uuid.Nil, err
				}
			}
			eloPlayers = append(eloPlayers, EloPlayer{UserID: *userID, Rank: player.Rank})
			ids, err := EvaluateAchievementIDs(tx, achievementContext{
				IsWinner:          player.IsWinner,
				SharedWin:         player.IsWinner && sharedWin,
				Penalty:           player.PenaltyPoints,
				GamesPlayed:       snap.GamesPlayed,
				Wins:              snap.Wins,
				CurrentStreak:     snap.CurrentStreak,
				CurrentTop2Streak: snap.CurrentTop2Streak,
				FirstPlaceCount:   snap.FirstPlaceCount,
				ZeroPenaltyGames:  snap.ZeroPenaltyGames,
				HumanOnlyGames:    snap.HumanOnlyGames,
			})
			if err != nil {
				return uuid.Nil, err
			}
			if err := AwardAchievements(tx, *userID, ids); err != nil {
				return uuid.Nil, err
			}
		}
	}

	// Skill rating (Phase B): adjust the registered players' ELO from their
	// finishing ranks via pairwise expansion. Needs >= 2 registered players to
	// have anything to compare; a lone human among bots simply doesn't move.
	eloDeltas := map[string]int{}
	if len(eloPlayers) >= 2 {
		eloDeltas, err = applyEloUpdates(tx, seasonID, eloPlayers)
		if err != nil {
			return uuid.Nil, err
		}
	}

	// Insert rating events for every registered player (even when delta is 0).
	// eloDeltas holds the lifetime delta, matching the user_stats.rating that
	// insertRatingEvent reads back, so before/after/delta stay self-consistent.
	for _, ep := range eloPlayers {
		delta := eloDeltas[ep.UserID.String()]
		if err := insertRatingEvent(tx, gameID, ep.UserID, delta); err != nil {
			return uuid.Nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("commit save game: %w", err)
	}
	committed = true
	return gameID, nil
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
		SELECT g.id, g.room_id, g.room_name, g.started_at, g.finished_at, gp.penalty_points, gp.rank, gp.is_winner
		FROM game_players gp
		JOIN games g ON g.id = gp.game_id
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
		if err := rows.Scan(&game.GameID, &game.RoomID, &roomName, &startedAt, &finishedAt, &game.PenaltyPoints, &game.Rank, &game.IsWinner); err != nil {
			return nil, 0, fmt.Errorf("scan history: %w", err)
		}
		game.RoomName = roomName.String
		game.StartedAt = startedAt.UTC().Format(time.RFC3339)
		game.FinishedAt = finishedAt.UTC().Format(time.RFC3339)
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
