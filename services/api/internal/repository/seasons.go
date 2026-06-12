package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Season is a named, time-windowed leaderboard bucket. ID is a UTC month bucket
// ('2026-06'); EndedAt is null for the single open/current season.
type Season struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Active    bool    `json:"active"`
}

// monthBucket returns the ('2026-06', 'June 2026') id/label pair for a time.
func monthBucket(t time.Time) (id, label string) {
	t = t.UTC()
	return t.Format("2006-01"), t.Format("January 2006")
}

// EnsureActiveSeason returns the currently active season, lazily rolling over to
// the current UTC month if the open season belongs to a past month. Rollover is
// idempotent: the prior open season is closed (ended_at = NOW()) and the new
// month's row is inserted with ON CONFLICT DO NOTHING, so concurrent callers
// converge on the same row. No cron is required — the first save or query in a
// new month performs the rollover.
func EnsureActiveSeason(db *sql.DB) (Season, error) {
	wantID, wantLabel := monthBucket(time.Now())

	// Fast path: the open season already matches the current month.
	var s Season
	var ended sql.NullString
	err := db.QueryRow(`
		SELECT id, label, started_at, ended_at
		FROM seasons
		WHERE ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&s.ID, &s.Label, &s.StartedAt, &ended)
	if err != nil && err != sql.ErrNoRows {
		return Season{}, fmt.Errorf("query active season: %w", err)
	}
	if err == nil && s.ID == wantID {
		s.Active = true
		return s, nil
	}

	// Rollover (or first-ever season): close any stale open season and open the
	// current month's. Done in one transaction so the "exactly one open season"
	// invariant holds.
	tx, txErr := db.Begin()
	if txErr != nil {
		return Season{}, fmt.Errorf("begin season rollover: %w", txErr)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
		UPDATE seasons SET ended_at = NOW()
		WHERE ended_at IS NULL AND id <> $1
	`, wantID); err != nil {
		return Season{}, fmt.Errorf("close stale season: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO seasons (id, label, started_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (id) DO UPDATE SET ended_at = NULL
	`, wantID, wantLabel); err != nil {
		return Season{}, fmt.Errorf("open current season: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Season{}, fmt.Errorf("commit season rollover: %w", err)
	}
	committed = true

	var startedAt string
	if err := db.QueryRow(`SELECT started_at FROM seasons WHERE id = $1`, wantID).Scan(&startedAt); err != nil {
		return Season{}, fmt.Errorf("read opened season: %w", err)
	}
	return Season{ID: wantID, Label: wantLabel, StartedAt: startedAt, Active: true}, nil
}

// ListSeasons returns all seasons newest-first for the leaderboard's season
// selector. The active (open) season is flagged.
func ListSeasons(db *sql.DB) ([]Season, error) {
	rows, err := db.Query(`
		SELECT id, label, started_at, ended_at
		FROM seasons
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list seasons: %w", err)
	}
	defer rows.Close()

	seasons := []Season{}
	for rows.Next() {
		var s Season
		var ended sql.NullString
		if err := rows.Scan(&s.ID, &s.Label, &s.StartedAt, &ended); err != nil {
			return nil, fmt.Errorf("scan season: %w", err)
		}
		if ended.Valid {
			s.EndedAt = &ended.String
		} else {
			s.Active = true
		}
		seasons = append(seasons, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate seasons: %w", err)
	}
	return seasons, nil
}

// UpsertSeasonUserStats increments a registered player's per-season counters
// inside the caller's transaction, mirroring UpsertUserStats. It does not touch
// rating (handled separately by ApplyRatingDeltas) so the two concerns stay
// independent. Guests/bots are skipped by the caller.
func UpsertSeasonUserStats(tx *sql.Tx, seasonID string, p UpsertUserStatsParams) error {
	winInc := 0
	top2Inc := 0
	if p.IsWinner {
		winInc = 1
		top2Inc = 1
	} else if p.Rank == 2 {
		top2Inc = 1
	}

	lowPenInc := 0
	if p.Penalty <= 5 {
		lowPenInc = 1
	}
	zeroPenInc := 0
	if p.Penalty == 0 {
		zeroPenInc = 1
	}
	highPenInc := 0
	if p.Penalty >= 20 {
		highPenInc = 1
	}

	humanInc := 0
	botInc := 0
	if p.HasBot {
		botInc = 1
	} else {
		humanInc = 1
	}

	closeWinInc := 0
	if p.CloseWin {
		closeWinInc = 1
	}
	closeLossInc := 0
	if p.CloseLoss {
		closeLossInc = 1
	}
	blowoutWinInc := 0
	if p.BlowoutWin {
		blowoutWinInc = 1
	}
	blowoutLossInc := 0
	if p.BlowoutLoss {
		blowoutLossInc = 1
	}

	_, err := tx.Exec(`
		INSERT INTO season_user_stats (
			season_id, user_id, games_played, wins, total_penalty, best_penalty,
			rank_sum, first_place_count, second_place_count, third_place_count, fourth_place_count,
			worst_penalty, zero_penalty_games, low_penalty_games, high_penalty_games,
			human_only_games, bot_mixed_games,
			current_streak, best_win_streak, current_top2_streak, best_top2_streak,
			close_wins, close_losses, blowout_wins, blowout_losses,
			rating, updated_at
		) VALUES (
			$1, $2, 1, $3, $4::bigint, $4::integer,
			$5, $6, $7, $8, $9,
			$4::integer, $10::integer, $11::integer, $12::integer,
			$13::integer, $14::integer,
			$3, $3, $15, $15,
			$16::integer, $17::integer, $18::integer, $19::integer,
			1200, NOW()
		)
		ON CONFLICT (season_id, user_id) DO UPDATE SET
			games_played       = season_user_stats.games_played + 1,
			wins               = season_user_stats.wins + $3,
			total_penalty      = season_user_stats.total_penalty + $4::bigint,
			best_penalty       = LEAST(COALESCE(season_user_stats.best_penalty, $4::integer), $4::integer),
			worst_penalty      = GREATEST(COALESCE(season_user_stats.worst_penalty, $4::integer), $4::integer),
			rank_sum           = season_user_stats.rank_sum + $5,
			first_place_count  = season_user_stats.first_place_count + $6,
			second_place_count = season_user_stats.second_place_count + $7,
			third_place_count  = season_user_stats.third_place_count + $8,
			fourth_place_count = season_user_stats.fourth_place_count + $9,
			zero_penalty_games = season_user_stats.zero_penalty_games + $10::integer,
			low_penalty_games  = season_user_stats.low_penalty_games + $11::integer,
			high_penalty_games = season_user_stats.high_penalty_games + $12::integer,
			human_only_games   = season_user_stats.human_only_games + $13::integer,
			bot_mixed_games    = season_user_stats.bot_mixed_games + $14::integer,
			current_streak     = CASE WHEN $3 = 1 THEN season_user_stats.current_streak + 1 ELSE 0 END,
			best_win_streak    = CASE WHEN $3 = 1 AND season_user_stats.current_streak + 1 > season_user_stats.best_win_streak THEN season_user_stats.current_streak + 1 ELSE season_user_stats.best_win_streak END,
			current_top2_streak = CASE WHEN $15 = 1 THEN season_user_stats.current_top2_streak + 1 ELSE 0 END,
			best_top2_streak   = CASE WHEN $15 = 1 AND season_user_stats.current_top2_streak + 1 > season_user_stats.best_top2_streak THEN season_user_stats.current_top2_streak + 1 ELSE season_user_stats.best_top2_streak END,
			close_wins         = season_user_stats.close_wins + $16::integer,
			close_losses       = season_user_stats.close_losses + $17::integer,
			blowout_wins       = season_user_stats.blowout_wins + $18::integer,
			blowout_losses     = season_user_stats.blowout_losses + $19::integer,
			updated_at         = NOW()
	`, seasonID, p.UserID, winInc, p.Penalty,
		p.Rank,
		boolToInt(p.Rank == 1), boolToInt(p.Rank == 2), boolToInt(p.Rank == 3), boolToInt(p.Rank == 4),
		zeroPenInc, lowPenInc, highPenInc,
		humanInc, botInc,
		top2Inc,
		closeWinInc, closeLossInc, blowoutWinInc, blowoutLossInc,
	)
	if err != nil {
		return fmt.Errorf("upsert season user stats: %w", err)
	}
	return nil
}

// ReadRatings fetches the current lifetime rating for each user id, defaulting
// to DefaultRating for users with no user_stats row yet (their first rated
// game). Used inside SaveGame's transaction to seed the ELO calculation.
func ReadRatings(tx *sql.Tx, userIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	ratings := make(map[uuid.UUID]int, len(userIDs))
	for _, id := range userIDs {
		ratings[id] = DefaultRating
	}
	for _, id := range userIDs {
		var r int
		err := tx.QueryRow(`SELECT rating FROM user_stats WHERE user_id = $1`, id).Scan(&r)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read rating: %w", err)
		}
		ratings[id] = r
	}
	return ratings, nil
}

// ReadSeasonRatings is ReadRatings for a season bucket.
func ReadSeasonRatings(tx *sql.Tx, seasonID string, userIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	ratings := make(map[uuid.UUID]int, len(userIDs))
	for _, id := range userIDs {
		ratings[id] = DefaultRating
	}
	for _, id := range userIDs {
		var r int
		err := tx.QueryRow(`SELECT rating FROM season_user_stats WHERE season_id = $1 AND user_id = $2`, seasonID, id).Scan(&r)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read season rating: %w", err)
		}
		ratings[id] = r
	}
	return ratings, nil
}

// ApplyRatingDelta adds delta to a user's lifetime rating. The user_stats row is
// guaranteed to exist because UpsertUserStats runs first in SaveGame; this is a
// plain UPDATE clamped at a floor of 0 to avoid negative ratings.
func ApplyRatingDelta(tx *sql.Tx, userID uuid.UUID, delta int) error {
	if _, err := tx.Exec(`
		UPDATE user_stats SET rating = GREATEST(0, rating + $2) WHERE user_id = $1
	`, userID, delta); err != nil {
		return fmt.Errorf("apply rating delta: %w", err)
	}
	return nil
}

// ApplySeasonRatingDelta is ApplyRatingDelta for the season bucket. The
// season_user_stats row exists because UpsertSeasonUserStats runs first.
func ApplySeasonRatingDelta(tx *sql.Tx, seasonID string, userID uuid.UUID, delta int) error {
	if _, err := tx.Exec(`
		UPDATE season_user_stats SET rating = GREATEST(0, rating + $3)
		WHERE season_id = $1 AND user_id = $2
	`, seasonID, userID, delta); err != nil {
		return fmt.Errorf("apply season rating delta: %w", err)
	}
	return nil
}
