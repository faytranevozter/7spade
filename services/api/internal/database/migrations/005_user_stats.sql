CREATE TABLE IF NOT EXISTS user_stats (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    games_played  INTEGER NOT NULL DEFAULT 0,
    wins          INTEGER NOT NULL DEFAULT 0,
    total_penalty BIGINT  NOT NULL DEFAULT 0,   -- sum of penalty_points; avg derived
    best_penalty  INTEGER NULL,                 -- lowest single-game penalty, null until first game
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Supports the min-games qualification filter and ordering scans.
CREATE INDEX IF NOT EXISTS idx_user_stats_games_played ON user_stats(games_played);

-- One-time backfill from existing recorded games (registered players only).
INSERT INTO user_stats (user_id, games_played, wins, total_penalty, best_penalty, updated_at)
SELECT
    gp.user_id,
    COUNT(*)                              AS games_played,
    COUNT(*) FILTER (WHERE gp.is_winner)  AS wins,
    COALESCE(SUM(gp.penalty_points), 0)   AS total_penalty,
    MIN(gp.penalty_points)                AS best_penalty,
    NOW()
FROM game_players gp
WHERE gp.user_id IS NOT NULL
GROUP BY gp.user_id
ON CONFLICT (user_id) DO NOTHING;
