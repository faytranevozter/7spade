-- Extended player stats: rank distribution, penalty buckets, streaks,
-- close-game stories, bot-vs-human splits, and rating event history.
-- Mirrored on both user_stats (all-time) and season_user_stats (per-season).

-- ============================================================================
-- user_stats (all-time)
-- ============================================================================
ALTER TABLE user_stats
    ADD COLUMN IF NOT EXISTS rank_sum           INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS first_place_count  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS second_place_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS third_place_count  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fourth_place_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS worst_penalty      INTEGER,
    ADD COLUMN IF NOT EXISTS zero_penalty_games INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS low_penalty_games  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS high_penalty_games INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS human_only_games   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bot_mixed_games    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS best_win_streak    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS best_top2_streak   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS current_top2_streak INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS close_wins         INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS close_losses       INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS blowout_wins       INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS blowout_losses     INTEGER NOT NULL DEFAULT 0;

-- ============================================================================
-- season_user_stats (per-season mirror)
-- ============================================================================
ALTER TABLE season_user_stats
    ADD COLUMN IF NOT EXISTS rank_sum           INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS first_place_count  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS second_place_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS third_place_count  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fourth_place_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS worst_penalty      INTEGER,
    ADD COLUMN IF NOT EXISTS zero_penalty_games INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS low_penalty_games  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS high_penalty_games INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS human_only_games   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bot_mixed_games    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS current_streak     INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS best_win_streak    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS best_top2_streak   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS current_top2_streak INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS close_wins         INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS close_losses       INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS blowout_wins       INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS blowout_losses     INTEGER NOT NULL DEFAULT 0;

-- ============================================================================
-- player_rating_events — per-game rating snapshot for profile charts
-- ============================================================================
CREATE TABLE IF NOT EXISTS player_rating_events (
    game_id        UUID      NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id        UUID      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating_before  INTEGER   NOT NULL,
    rating_after   INTEGER   NOT NULL,
    rating_delta   INTEGER   NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, user_id)
);

-- ============================================================================
-- game_players — add is_bot flag so stats can distinguish human-only games
-- ============================================================================
ALTER TABLE game_players
    ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;

-- ============================================================================
-- Backfill: existing game_players rows don't have is_bot set, but the flag
-- defaults to FALSE which matches reality (all past games were recorded before
-- the bot split existed).
--
-- The rate-driving counters (rank_sum + placement counts) MUST be backfilled
-- from history, because the read path divides them by the pre-existing,
-- cumulative games_played (avg_rank = rank_sum/games_played, top2_rate =
-- (first+second)/games_played). Leaving them at 0 while games_played already
-- counts every historical game would permanently understate those rates for
-- legacy accounts and corrupt the avg_rank leaderboard sort. Penalty buckets and
-- worst_penalty are reconstructable too, so we backfill them for consistency.
--
-- Streaks and close/blowout counters are plain counts (never divided), so a 0
-- start introduces no rate skew; they are intentionally left to accrue from new
-- games rather than reconstructed (close/blowout need per-game margins not
-- cheaply available here).
-- ============================================================================
WITH agg AS (
    SELECT
        gp.user_id,
        COALESCE(SUM(gp.rank), 0)                              AS rank_sum,
        COUNT(*) FILTER (WHERE gp.rank = 1)                    AS first_place_count,
        COUNT(*) FILTER (WHERE gp.rank = 2)                    AS second_place_count,
        COUNT(*) FILTER (WHERE gp.rank = 3)                    AS third_place_count,
        COUNT(*) FILTER (WHERE gp.rank = 4)                    AS fourth_place_count,
        MAX(gp.penalty_points)                                 AS worst_penalty,
        COUNT(*) FILTER (WHERE gp.penalty_points = 0)          AS zero_penalty_games,
        COUNT(*) FILTER (WHERE gp.penalty_points <= 5)         AS low_penalty_games,
        COUNT(*) FILTER (WHERE gp.penalty_points >= 20)        AS high_penalty_games
    FROM game_players gp
    WHERE gp.user_id IS NOT NULL
    GROUP BY gp.user_id
)
UPDATE user_stats us SET
    rank_sum           = agg.rank_sum,
    first_place_count  = agg.first_place_count,
    second_place_count = agg.second_place_count,
    third_place_count  = agg.third_place_count,
    fourth_place_count = agg.fourth_place_count,
    worst_penalty      = agg.worst_penalty,
    zero_penalty_games = agg.zero_penalty_games,
    low_penalty_games  = agg.low_penalty_games,
    high_penalty_games = agg.high_penalty_games
FROM agg
WHERE agg.user_id = us.user_id;
