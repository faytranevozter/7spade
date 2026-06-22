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
	UserID       string  `json:"user_id"`
	DisplayName  string  `json:"display_name"`
	AvatarURL    *string `json:"avatar_url"`
	GamesPlayed  int     `json:"games_played"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"win_rate"`
	AvgPenalty   float64 `json:"avg_penalty"`
	BestPenalty  *int    `json:"best_penalty"`
	WorstPenalty *int    `json:"worst_penalty"`
	Rating       int     `json:"rating"`
	Rank         *int    `json:"rank"`
	Qualified    bool    `json:"qualified"`

	AvgRank          float64 `json:"avg_rank"`
	FirstPlaceCount  int     `json:"first_place_count"`
	SecondPlaceCount int     `json:"second_place_count"`
	ThirdPlaceCount  int     `json:"third_place_count"`
	FourthPlaceCount int     `json:"fourth_place_count"`

	ZeroPenaltyGames int `json:"zero_penalty_games"`
	LowPenaltyGames  int `json:"low_penalty_games"`
	HighPenaltyGames int `json:"high_penalty_games"`

	HumanOnlyGames int `json:"human_only_games"`
	BotMixedGames  int `json:"bot_mixed_games"`

	CurrentWinStreak  int `json:"current_win_streak"`
	BestWinStreak     int `json:"best_win_streak"`
	CurrentTop2Streak int `json:"current_top2_streak"`
	BestTop2Streak    int `json:"best_top2_streak"`

	CloseWins     int `json:"close_wins"`
	CloseLosses   int `json:"close_losses"`
	BlowoutWins   int `json:"blowout_wins"`
	BlowoutLosses int `json:"blowout_losses"`

	// Lifetime XP and derived level progression. Level/progress fields are
	// computed from XP at read time (see fillXP). Season-scoped reads leave XP
	// at 0 / level 1, since XP is lifetime-only.
	XP             int64 `json:"xp"`
	Level          int   `json:"level"`
	XPIntoLevel    int64 `json:"xp_into_level"`
	XPForNextLevel int64 `json:"xp_for_next_level"`
	XPToNextLevel  int64 `json:"xp_to_next_level"`
}

// fillXP populates the derived level/progress fields from the stored XP total.
// Call after XP is scanned so the API always returns a consistent level.
func (s *UserStats) fillXP() {
	s.Level, s.XPIntoLevel, s.XPForNextLevel, s.XPToNextLevel = XPProgress(s.XP)
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

	AvgRank         float64 `json:"avg_rank"`
	Top2Rate        float64 `json:"top2_rate"`
	FirstPlaceCount int     `json:"first_place_count"`
	HumanOnlyGames  int     `json:"human_only_games"`
	BotMixedGames   int     `json:"bot_mixed_games"`

	XP    int64 `json:"xp"`
	Level int   `json:"level"`
}

// StatsSnapshot is the post-update view of a player's counters returned by
// UpsertUserStats, so the achievement evaluator can decide what to award
// without a second read inside the same transaction.
type StatsSnapshot struct {
	GamesPlayed       int
	Wins              int
	CurrentStreak     int
	CurrentTop2Streak int
	BestWinStreak     int
	BestTop2Streak    int
	FirstPlaceCount   int
	ZeroPenaltyGames  int
	HumanOnlyGames    int
	XP                int64 // lifetime XP after this game's award
}

// UpsertUserStatsParams carries everything UpsertUserStats needs to update
// a single player's row after a non-practice game.
type UpsertUserStatsParams struct {
	UserID      uuid.UUID
	IsWinner    bool
	Penalty     int
	Rank        int
	HasBot      bool // the game included at least one bot
	CloseWin    bool // margin to next-best <= 3
	CloseLoss   bool // margin to winner <= 3 (non-winner only)
	BlowoutWin  bool // margin to next-best >= 15
	BlowoutLoss bool // margin from winner >= 15 (last place only)
	XPDelta     int  // lifetime XP awarded for this game (UpsertUserStats only)
}

// UpsertUserStats increments a registered player's lifetime counters inside the
// caller's transaction. Returns the post-update counters including streaks.
// Must be called once per registered player from within SaveGame's transaction;
// guests/bots (empty UserID) are skipped by the caller.
func UpsertUserStats(tx *sql.Tx, p UpsertUserStatsParams) (StatsSnapshot, error) {
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

	var snap StatsSnapshot
	err := tx.QueryRow(`
		INSERT INTO user_stats (
			user_id, games_played, wins, total_penalty, best_penalty,
			rank_sum, first_place_count, second_place_count, third_place_count, fourth_place_count,
			worst_penalty, zero_penalty_games, low_penalty_games, high_penalty_games,
			human_only_games, bot_mixed_games,
			current_streak, best_win_streak, current_top2_streak, best_top2_streak,
			close_wins, close_losses, blowout_wins, blowout_losses,
			rating, xp, updated_at
		) VALUES (
			$1, 1, $2, $3::bigint, $3::integer,
			$4, $5, $6, $7, $8,
			$3::integer, $9::integer, $10::integer, $11::integer,
			$12::integer, $13::integer,
			$2, $2, $14, $14,
			$15::integer, $16::integer, $17::integer, $18::integer,
			1200, $19::bigint, NOW()
		)
		ON CONFLICT (user_id) DO UPDATE SET
			games_played       = user_stats.games_played + 1,
			wins               = user_stats.wins + $2,
			total_penalty      = user_stats.total_penalty + $3::bigint,
			best_penalty       = LEAST(COALESCE(user_stats.best_penalty, $3::integer), $3::integer),
			worst_penalty      = GREATEST(COALESCE(user_stats.worst_penalty, $3::integer), $3::integer),
			rank_sum           = user_stats.rank_sum + $4,
			first_place_count  = user_stats.first_place_count + $5,
			second_place_count = user_stats.second_place_count + $6,
			third_place_count  = user_stats.third_place_count + $7,
			fourth_place_count = user_stats.fourth_place_count + $8,
			zero_penalty_games = user_stats.zero_penalty_games + $9::integer,
			low_penalty_games  = user_stats.low_penalty_games + $10::integer,
			high_penalty_games = user_stats.high_penalty_games + $11::integer,
			human_only_games   = user_stats.human_only_games + $12::integer,
			bot_mixed_games    = user_stats.bot_mixed_games + $13::integer,
			current_streak     = CASE WHEN $2 = 1 THEN user_stats.current_streak + 1 ELSE 0 END,
			best_win_streak    = CASE WHEN $2 = 1 AND user_stats.current_streak + 1 > user_stats.best_win_streak THEN user_stats.current_streak + 1 ELSE user_stats.best_win_streak END,
			current_top2_streak = CASE WHEN $14 = 1 THEN user_stats.current_top2_streak + 1 ELSE 0 END,
			best_top2_streak   = CASE WHEN $14 = 1 AND user_stats.current_top2_streak + 1 > user_stats.best_top2_streak THEN user_stats.current_top2_streak + 1 ELSE user_stats.best_top2_streak END,
			close_wins         = user_stats.close_wins + $15::integer,
			close_losses       = user_stats.close_losses + $16::integer,
			blowout_wins       = user_stats.blowout_wins + $17::integer,
			blowout_losses     = user_stats.blowout_losses + $18::integer,
			xp                 = user_stats.xp + $19::bigint,
			updated_at         = NOW()
		RETURNING games_played, wins, current_streak, current_top2_streak, best_win_streak, best_top2_streak,
		          first_place_count, zero_penalty_games, human_only_games, xp
	`, p.UserID, winInc, p.Penalty,
		p.Rank,
		boolToInt(p.Rank == 1), boolToInt(p.Rank == 2), boolToInt(p.Rank == 3), boolToInt(p.Rank == 4),
		zeroPenInc, lowPenInc, highPenInc,
		humanInc, botInc,
		top2Inc,
		closeWinInc, closeLossInc, blowoutWinInc, blowoutLossInc,
		p.XPDelta,
	).Scan(&snap.GamesPlayed, &snap.Wins, &snap.CurrentStreak, &snap.CurrentTop2Streak, &snap.BestWinStreak, &snap.BestTop2Streak,
		&snap.FirstPlaceCount, &snap.ZeroPenaltyGames, &snap.HumanOnlyGames, &snap.XP)
	if err != nil {
		return StatsSnapshot{}, fmt.Errorf("upsert user stats: %w", err)
	}
	return snap, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
	"avg_rank": `
		ORDER BY (rank_sum::float8 / games_played) ASC,
		         games_played DESC,
		         human_only_games DESC,
		         user_id ASC
	`,
	"top2_rate": `
		ORDER BY ((first_place_count + second_place_count)::float8 / games_played) DESC,
		         games_played DESC,
		         human_only_games DESC,
		         user_id ASC
	`,
	// xp is lifetime-only; the column exists only on user_stats, so this sort
	// must never be applied to a season-scoped query (guarded in GetLeaderboard).
	"xp": `
		ORDER BY xp DESC,
		         games_played DESC,
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
	// XP is lifetime-only: the column lives on user_stats, not season_user_stats.
	// A season-scoped xp sort would reference a missing column, so coerce it back
	// to the default ordering for season views.
	if sort == "xp" && seasonID != "" {
		sort = DefaultLeaderboardSort
	}
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

	// XP lives only on user_stats; season rows report 0 (level 1) since XP is
	// lifetime-only and not mirrored per season.
	xpCol := "s.xp"
	if seasonID != "" {
		xpCol = "0::bigint"
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
			s.rating,
			s.rank_sum::float8 / s.games_played  AS avg_rank,
			(s.first_place_count + s.second_place_count)::float8 / s.games_played AS top2_rate,
			s.first_place_count,
			s.human_only_games,
			s.bot_mixed_games,
			%s AS xp
		FROM %s
		JOIN users u ON u.id = s.user_id%s
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, order, xpCol, from, avatarLateralJoin, seasonWhere, order, limitIdx, offsetIdx)

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
		if err := rows.Scan(&e.Rank, &e.UserID, &e.DisplayName, &avatar, &e.GamesPlayed, &e.Wins, &e.WinRate, &e.AvgPenalty, &best, &e.Rating, &e.AvgRank, &e.Top2Rate, &e.FirstPlaceCount, &e.HumanOnlyGames, &e.BotMixedGames, &e.XP); err != nil {
			return nil, 0, "", fmt.Errorf("scan leaderboard: %w", err)
		}
		if avatar.Valid {
			e.AvatarURL = &avatar.String
		}
		if best.Valid {
			v := int(best.Int64)
			e.BestPenalty = &v
		}
		e.Level = LevelFromXP(e.XP)
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
		worst        sql.NullInt64
		totalPenalty int64
		rankSum      int64
		avatar       sql.NullString
		xp           int64 // lifetime xp; stays 0 for season-scoped reads
	)

	table := "user_stats"
	if seasonID != "" {
		table = "season_user_stats"
	}

	var row *sql.Row
	if seasonID == "" {
		row = db.QueryRow(`
			SELECT s.user_id, u.display_name, av.avatar_url,
			       s.games_played, s.wins, s.total_penalty, s.best_penalty, s.worst_penalty, s.rating,
			       s.rank_sum, s.first_place_count, s.second_place_count, s.third_place_count, s.fourth_place_count,
			       s.zero_penalty_games, s.low_penalty_games, s.high_penalty_games,
			       s.human_only_games, s.bot_mixed_games,
			       s.current_streak, s.best_win_streak, s.current_top2_streak, s.best_top2_streak,
			       s.close_wins, s.close_losses, s.blowout_wins, s.blowout_losses,
			       s.xp
			FROM user_stats s
			JOIN users u ON u.id = s.user_id`+avatarLateralJoin+`
			WHERE s.user_id = $1
		`, userID)
	} else {
		row = db.QueryRow(`
			SELECT s.user_id, u.display_name, av.avatar_url,
			       s.games_played, s.wins, s.total_penalty, s.best_penalty, s.worst_penalty, s.rating,
			       s.rank_sum, s.first_place_count, s.second_place_count, s.third_place_count, s.fourth_place_count,
			       s.zero_penalty_games, s.low_penalty_games, s.high_penalty_games,
			       s.human_only_games, s.bot_mixed_games,
			       s.current_streak, s.best_win_streak, s.current_top2_streak, s.best_top2_streak,
			       s.close_wins, s.close_losses, s.blowout_wins, s.blowout_losses,
			       0::bigint AS xp
			FROM season_user_stats s
			JOIN users u ON u.id = s.user_id`+avatarLateralJoin+`
			WHERE s.user_id = $1 AND s.season_id = $2
		`, userID, seasonID)
	}

	err := row.Scan(
		&stats.UserID, &stats.DisplayName, &avatar,
		&stats.GamesPlayed, &stats.Wins, &totalPenalty, &best, &worst, &stats.Rating,
		&rankSum,
		&stats.FirstPlaceCount, &stats.SecondPlaceCount, &stats.ThirdPlaceCount, &stats.FourthPlaceCount,
		&stats.ZeroPenaltyGames, &stats.LowPenaltyGames, &stats.HighPenaltyGames,
		&stats.HumanOnlyGames, &stats.BotMixedGames,
		&stats.CurrentWinStreak, &stats.BestWinStreak, &stats.CurrentTop2Streak, &stats.BestTop2Streak,
		&stats.CloseWins, &stats.CloseLosses, &stats.BlowoutWins, &stats.BlowoutLosses,
		&xp,
	)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query user stats: %w", err)
	}

	stats.XP = xp
	stats.fillXP()

	if avatar.Valid {
		stats.AvatarURL = &avatar.String
	}
	if best.Valid {
		v := int(best.Int64)
		stats.BestPenalty = &v
	}
	if worst.Valid {
		v := int(worst.Int64)
		stats.WorstPenalty = &v
	}
	if stats.GamesPlayed > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed)
		stats.AvgPenalty = float64(totalPenalty) / float64(stats.GamesPlayed)
		stats.AvgRank = float64(rankSum) / float64(stats.GamesPlayed)
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
