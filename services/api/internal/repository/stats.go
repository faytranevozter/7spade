package repository

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// UserStats is the read/DTO shape for a single user's lifetime stats. Derived
// fields (win_rate, avg_penalty) are computed from the stored integer counters
// at read time to avoid rounding drift. Rank and Qualified are populated by the
// handler / query depending on the leaderboard threshold.
type UserStats struct {
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	GamesPlayed int     `json:"games_played"`
	Wins        int     `json:"wins"`
	WinRate     float64 `json:"win_rate"`
	AvgPenalty  float64 `json:"avg_penalty"`
	BestPenalty *int    `json:"best_penalty"`
	Rank        *int    `json:"rank"`
	Qualified   bool    `json:"qualified"`
}

// LeaderboardEntry is one ranked row in the public leaderboard.
type LeaderboardEntry struct {
	Rank        int     `json:"rank"`
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	GamesPlayed int     `json:"games_played"`
	Wins        int     `json:"wins"`
	WinRate     float64 `json:"win_rate"`
	AvgPenalty  float64 `json:"avg_penalty"`
	BestPenalty *int    `json:"best_penalty"`
}

// StatsSnapshot is the post-update view of a player's counters returned by
// UpsertUserStats, so the achievement evaluator can decide what to award
// without a second read inside the same transaction.
type StatsSnapshot struct {
	GamesPlayed   int
	Wins          int
	CurrentStreak int
}

// UpsertUserStats increments a registered player's lifetime counters inside the
// caller's transaction. isWinner adds 1 to wins when true; penalty is the
// game's penalty_points, accumulated into total_penalty and folded into
// best_penalty via LEAST. current_streak increments on a win and resets to 0 on
// a loss. Returns the post-update counters. Must be called once per registered
// player from within SaveGame's transaction; guests/bots (empty UserID) are
// skipped by the caller.
func UpsertUserStats(tx *sql.Tx, userID uuid.UUID, isWinner bool, penalty int) (StatsSnapshot, error) {
	winInc := 0
	if isWinner {
		winInc = 1
	}
	// On a loss the streak resets to 0; on a win it increments. The CASE keeps
	// this in one statement so the counter never diverges from the wins tally.
	var snap StatsSnapshot
	err := tx.QueryRow(`
		INSERT INTO user_stats (user_id, games_played, wins, total_penalty, best_penalty, current_streak, updated_at)
		VALUES ($1, 1, $2, $3::bigint, $3::integer, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			games_played   = user_stats.games_played + 1,
			wins           = user_stats.wins + $2,
			total_penalty  = user_stats.total_penalty + $3::bigint,
			best_penalty   = LEAST(COALESCE(user_stats.best_penalty, $3::integer), $3::integer),
			current_streak = CASE WHEN $2 = 1 THEN user_stats.current_streak + 1 ELSE 0 END,
			updated_at     = NOW()
		RETURNING games_played, wins, current_streak
	`, userID, winInc, penalty).Scan(&snap.GamesPlayed, &snap.Wins, &snap.CurrentStreak)
	if err != nil {
		return StatsSnapshot{}, fmt.Errorf("upsert user stats: %w", err)
	}
	return snap, nil
}

// leaderboardOrder is the canonical ranking rule, shared by the page query and
// the rank-of-one-user query so they stay consistent: win_rate desc, then
// games_played desc, then avg_penalty asc (lower is better), then user_id for
// stable ordering.
const leaderboardOrder = `
	ORDER BY (wins::float8 / games_played) DESC,
	         games_played DESC,
	         (total_penalty::float8 / games_played) ASC,
	         user_id ASC
`

// GetLeaderboard returns a page of qualifying players (games_played >= minGames)
// ranked by the canonical ordering, plus the total count of qualifiers. rank is
// the 1-based position across the full qualifying set (not just the page).
func GetLeaderboard(db *sql.DB, page, perPage, minGames int) ([]LeaderboardEntry, int, error) {
	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM user_stats WHERE games_played >= $1`, minGames,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count leaderboard: %w", err)
	}

	query := `
		SELECT
			ROW_NUMBER() OVER (` + leaderboardOrder + `) AS rank,
			s.user_id,
			u.display_name,
			av.avatar_url,
			s.games_played,
			s.wins,
			s.wins::float8 / s.games_played      AS win_rate,
			s.total_penalty::float8 / s.games_played AS avg_penalty,
			s.best_penalty
		FROM user_stats s
		JOIN users u ON u.id = s.user_id` + avatarLateralJoin + `
		WHERE s.games_played >= $1
		` + leaderboardOrder + `
		LIMIT $2 OFFSET $3
	`
	rows, err := db.Query(query, minGames, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()

	entries := []LeaderboardEntry{}
	for rows.Next() {
		var e LeaderboardEntry
		var best sql.NullInt64
		var avatar sql.NullString
		if err := rows.Scan(&e.Rank, &e.UserID, &e.DisplayName, &avatar, &e.GamesPlayed, &e.Wins, &e.WinRate, &e.AvgPenalty, &best); err != nil {
			return nil, 0, fmt.Errorf("scan leaderboard: %w", err)
		}
		if avatar.Valid {
			e.AvatarURL = &avatar.String
		}
		if best.Valid {
			v := int(best.Int64)
			e.BestPenalty = &v
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate leaderboard: %w", err)
	}
	return entries, total, nil
}

// GetUserStats fetches a single user's stats and computes their leaderboard
// rank. The bool return is false when the user has no user_stats row (never
// played a recorded game). When the user qualifies (games_played >= minGames),
// Rank is set to their 1-based position and Qualified is true; otherwise Rank
// is nil and Qualified is false. display_name is read live from users.
func GetUserStats(db *sql.DB, userID uuid.UUID, minGames int) (*UserStats, bool, error) {
	var (
		stats        UserStats
		best         sql.NullInt64
		totalPenalty int64
		avatar       sql.NullString
	)
	err := db.QueryRow(`
		SELECT s.user_id, u.display_name, av.avatar_url, s.games_played, s.wins, s.total_penalty, s.best_penalty
		FROM user_stats s
		JOIN users u ON u.id = s.user_id`+avatarLateralJoin+`
		WHERE s.user_id = $1
	`, userID).Scan(&stats.UserID, &stats.DisplayName, &avatar, &stats.GamesPlayed, &stats.Wins, &totalPenalty, &best)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query user stats: %w", err)
	}

	if avatar.Valid {
		stats.AvatarURL = &avatar.String
	}
	if best.Valid {
		v := int(best.Int64)
		stats.BestPenalty = &v
	}
	if stats.GamesPlayed > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed)
		stats.AvgPenalty = float64(totalPenalty) / float64(stats.GamesPlayed)
	}

	if stats.GamesPlayed >= minGames {
		// Rank = number of qualifying users strictly ahead of this user under
		// the canonical ordering, plus one. The target user's win_rate and
		// avg_penalty are recomputed inside Postgres (via the `me` row) rather
		// than passed in as Go-computed floats, so both sides of the tie-break
		// comparisons use identical server-side float arithmetic. This keeps the
		// rank here bit-for-bit consistent with the ROW_NUMBER() ordering in
		// GetLeaderboard even for mathematically-equal win rates expressed as
		// different fractions.
		var ahead int
		if err := db.QueryRow(`
			SELECT COUNT(*)
			FROM user_stats s
			CROSS JOIN (
				SELECT wins, games_played, total_penalty
				FROM user_stats
				WHERE user_id = $1
			) me
			WHERE s.games_played >= $2
			  AND s.user_id <> $1
			  AND (
			        (s.wins::float8 / s.games_played) > (me.wins::float8 / me.games_played)
			     OR ((s.wins::float8 / s.games_played) = (me.wins::float8 / me.games_played)
			         AND s.games_played > me.games_played)
			     OR ((s.wins::float8 / s.games_played) = (me.wins::float8 / me.games_played)
			         AND s.games_played = me.games_played
			         AND (s.total_penalty::float8 / s.games_played) < (me.total_penalty::float8 / me.games_played))
			     OR ((s.wins::float8 / s.games_played) = (me.wins::float8 / me.games_played)
			         AND s.games_played = me.games_played
			         AND (s.total_penalty::float8 / s.games_played) = (me.total_penalty::float8 / me.games_played)
			         AND s.user_id < $1)
			      )
		`, userID, minGames).Scan(&ahead); err != nil {
			return nil, false, fmt.Errorf("rank user stats: %w", err)
		}
		rank := ahead + 1
		stats.Rank = &rank
		stats.Qualified = true
	}

	return &stats, true, nil
}
