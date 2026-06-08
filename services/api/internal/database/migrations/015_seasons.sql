-- Time-windowed leaderboards. A season is a named month-long window; stats are
-- bucketed per season so a leaderboard can be scoped to one. The all-time
-- leaderboard (user_stats) is unchanged and remains the default.
CREATE TABLE IF NOT EXISTS seasons (
    id         TEXT PRIMARY KEY,          -- date bucket, e.g. '2026-06'
    label      TEXT NOT NULL,             -- human label, e.g. 'June 2026'
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at   TIMESTAMP NULL             -- null = current/open season
);

-- Mirrors user_stats but keyed by (season_id, user_id). rating carries the
-- per-season ELO (Phase B); defaults match the lifetime rating default.
CREATE TABLE IF NOT EXISTS season_user_stats (
    season_id     TEXT    NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    user_id       UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    games_played  INTEGER NOT NULL DEFAULT 0,
    wins          INTEGER NOT NULL DEFAULT 0,
    total_penalty BIGINT  NOT NULL DEFAULT 0,   -- sum of penalty_points; avg derived
    best_penalty  INTEGER NULL,                 -- lowest single-game penalty for the season
    rating        INTEGER NOT NULL DEFAULT 1200,
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (season_id, user_id)
);

-- Supports the min-games qualification filter and per-season ordering scans.
CREATE INDEX IF NOT EXISTS idx_season_user_stats_lookup
    ON season_user_stats(season_id, games_played);

-- Seed the current month's open season so the active-season resolver has a row
-- to find on first run. id/label use the UTC month; subsequent seasons are
-- created lazily by EnsureActiveSeason on the first save/query of a new month.
INSERT INTO seasons (id, label, started_at)
VALUES (
    to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM'),
    to_char(NOW() AT TIME ZONE 'UTC', 'FMMonth YYYY'),
    date_trunc('month', NOW() AT TIME ZONE 'UTC')
)
ON CONFLICT (id) DO NOTHING;
