CREATE TABLE IF NOT EXISTS daily_login_xp_events (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    claim_date  DATE NOT NULL,
    streak_day  INTEGER NOT NULL CHECK (streak_day >= 1),
    xp_before   BIGINT NOT NULL,
    xp_after    BIGINT NOT NULL,
    xp_delta    INTEGER NOT NULL CHECK (xp_delta >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, claim_date)
);

CREATE INDEX IF NOT EXISTS daily_login_xp_events_user_created_idx
    ON daily_login_xp_events (user_id, created_at DESC);
