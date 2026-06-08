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
	Rating      int     `json:"rating"`
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
	Rating      int     `json:"rating"`
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

// DefaultLeaderboardSort is the sort key applied when none (or an unknown one)
// is requested. It matches the historical win_rate-first ordering.
const DefaultLeaderboardSort = "win_rate"

// leaderboardOrders is an allowlist of sort keys to their ORDER BY fragment.
// Each fragment carries stable secondary keys ending in `user_id ASC` so the
// ranking is deterministic. The map is the only source of ordering SQL — the
// requested sort string is never interpolated into the query — so there is no
// injection surface. win_rate preserves the historical canonical ordering.
var leaderboardOrders = map[string]string{
	"win_rate": `
		ORDER BY (wins::float8 / games_played) DESC,
		         games_played DESC,
		         (total_penalty::float8 / games_played) ASC,
		         user_id ASC
	`,
	"total_wins": `
		ORDER BY wins DESC,
		         (wins::float8 / games_played) DESC,
		         user_id ASC
	`,
	"avg_penalty": `
		ORDER BY (total_penalty::float8 / games_played) ASC,
		         games_played DESC,
		         user_id ASC
	`,
	"best_penalty": `
		ORDER BY best_penalty ASC NULLS LAST,
		         (total_penalty::float8 / games_played) ASC,
		         user_id ASC
	`,
	"games_played": `
		ORDER BY games_played DESC,
		         wins DESC,
		         user_id ASC
	`,
	"rating": `
		ORDER BY rating DESC,
		         games_played DESC,
		         (wins::float8 / games_played) DESC,
		         user_id ASC
	`,
}

// leaderboardOrderFor resolves a requested sort key to its ORDER BY fragment,
// falling back to DefaultLeaderboardSort for empty or unknown keys. It returns
// the fragment plus the normalized key actually applied (for the API to echo
// back so the client can sync its UI state).
func leaderboardOrderFor(sort string) (clause, normalized string) {
	if clause, ok := leaderboardOrders[sort]; ok {
		return clause, sort
	}
	return leaderboardOrders[DefaultLeaderboardSort], DefaultLeaderboardSort
}

// GetLeaderboard returns a page of qualifying players (games_played >= minGames)
// ranked by the requested sort, plus the total count of qualifiers and the
// normalized sort key actually applied. rank is the 1-based position across the
// full qualifying set (not just the page) under the same ordering.
//
// seasonID scopes the query: empty selects the all-time table (user_stats);
// a non-empty id selects that season's bucket (season_user_stats). Both tables
// share the same columns (incl. rating), so the ranking rule and threshold are
// identical either way.
func GetLeaderboard(db *sql.DB, page, perPage, minGames int, sort, seasonID string) ([]LeaderboardEntry, int, string, error) {
	order, normalizedSort := leaderboardOrderFor(sort)

	// Source table + leading args differ only by season scope. For a season the
	// season_id is the first positional arg so the remaining placeholders keep
	// their meaning; the all-time path has no extra arg.
	var (
		from        string
		countQuery  string
		countArgs   []any
		seasonWhere string
		baseArgs    []any // args before LIMIT/OFFSET, in placeholder order
	)
	if seasonID == "" {
		from = "user_stats s"
		countQuery = `SELECT COUNT(*) FROM user_stats WHERE games_played >= $1`
		countArgs = []any{minGames}
		seasonWhere = `WHERE s.games_played >= $1`
		baseArgs = []any{minGames}
	} else {
		from = "season_user_stats s"
		countQuery = `SELECT COUNT(*) FROM season_user_stats WHERE season_id = $1 AND games_played >= $2`
		countArgs = []any{seasonID, minGames}
		seasonWhere = `WHERE s.season_id = $1 AND s.games_played >= $2`
		baseArgs = []any{seasonID, minGames}
	}

	var total int
	if err := db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, "", fmt.Errorf("count leaderboard: %w", err)
	}

	limitIdx := len(baseArgs) + 1
	offsetIdx := len(baseArgs) + 2
	query := fmt.Sprintf(`
		SELECT
			ROW_NUMBER() OVER (%s) AS rank,
			s.user_id,
			u.display_name,
			av.avatar_url,
			s.games_played,
			s.wins,
			s.wins::float8 / s.games_played      AS win_rate,
			s.total_penalty::float8 / s.games_played AS avg_penalty,
			s.best_penalty,
			s.rating
		FROM %s
		JOIN users u ON u.id = s.user_id%s
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, order, from, avatarLateralJoin, seasonWhere, order, limitIdx, offsetIdx)

	args := append(append([]any{}, baseArgs...), perPage, (page-1)*perPage)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, "", fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()

	entries := []LeaderboardEntry{}
	for rows.Next() {
		var e LeaderboardEntry
		var best sql.NullInt64
		var avatar sql.NullString
		if err := rows.Scan(&e.Rank, &e.UserID, &e.DisplayName, &avatar, &e.GamesPlayed, &e.Wins, &e.WinRate, &e.AvgPenalty, &best, &e.Rating); err != nil {
			return nil, 0, "", fmt.Errorf("scan leaderboard: %w", err)
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
		return nil, 0, "", fmt.Errorf("iterate leaderboard: %w", err)
	}
	return entries, total, normalizedSort, nil
}

// GetUserStats fetches a single user's stats and computes their leaderboard
// rank. The bool return is false when the user has no stats row (never played a
// recorded game in scope). When the user qualifies (games_played >= minGames),
// Rank is set to their 1-based position and Qualified is true; otherwise Rank
// is nil and Qualified is false. display_name is read live from users.
//
// seasonID scopes the stats: empty reads all-time (user_stats); a non-empty id
// reads that season's bucket (season_user_stats). The ranking comparison uses
// the matching table so rank stays consistent with GetLeaderboard.
func GetUserStats(db *sql.DB, userID uuid.UUID, minGames int, seasonID string) (*UserStats, bool, error) {
	var (
		stats        UserStats
		best         sql.NullInt64
		totalPenalty int64
		avatar       sql.NullString
	)

	table := "user_stats"
	if seasonID != "" {
		table = "season_user_stats"
	}

	var row *sql.Row
	if seasonID == "" {
		row = db.QueryRow(`
			SELECT s.user_id, u.display_name, av.avatar_url, s.games_played, s.wins, s.total_penalty, s.best_penalty, s.rating
			FROM user_stats s
			JOIN users u ON u.id = s.user_id`+avatarLateralJoin+`
			WHERE s.user_id = $1
		`, userID)
	} else {
		row = db.QueryRow(`
			SELECT s.user_id, u.display_name, av.avatar_url, s.games_played, s.wins, s.total_penalty, s.best_penalty, s.rating
			FROM season_user_stats s
			JOIN users u ON u.id = s.user_id`+avatarLateralJoin+`
			WHERE s.user_id = $1 AND s.season_id = $2
		`, userID, seasonID)
	}

	err := row.Scan(&stats.UserID, &stats.DisplayName, &avatar, &stats.GamesPlayed, &stats.Wins, &totalPenalty, &best, &stats.Rating)
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
		args := []any{userID, minGames}
		if seasonID != "" {
			args = append(args, seasonID)
		}
		query := fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %[1]s s
			CROSS JOIN (
				SELECT wins, games_played, total_penalty
				FROM %[1]s
				WHERE user_id = $1 %[2]s
			) me
			WHERE s.games_played >= $2
			  AND s.user_id <> $1 %[2]s
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
		`, table, meSeasonFilter(seasonID, "$3"))
		if err := db.QueryRow(query, args...).Scan(&ahead); err != nil {
			return nil, false, fmt.Errorf("rank user stats: %w", err)
		}
		rank := ahead + 1
		stats.Rank = &rank
		stats.Qualified = true
	}

	return &stats, true, nil
}

// meSeasonFilter returns the season filter applied to the `me` sub-select inside
// the rank query when scoped to a season, or empty for the all-time table.
func meSeasonFilter(seasonID, placeholder string) string {
	if seasonID == "" {
		return ""
	}
	return "AND season_id = " + placeholder
}
