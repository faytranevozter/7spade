-- Experience points (XP) and level progression for registered players.
-- XP is lifetime-only (no season mirror); level is derived from XP at read time.

-- ============================================================================
-- user_stats — lifetime XP counter
-- ============================================================================
ALTER TABLE user_stats
    ADD COLUMN IF NOT EXISTS xp BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_user_stats_xp ON user_stats(xp DESC);

-- ============================================================================
-- player_xp_events — per-game XP snapshot for audit / profile history
-- ============================================================================
CREATE TABLE IF NOT EXISTS player_xp_events (
    game_id    UUID      NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id    UUID      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    xp_before  BIGINT    NOT NULL,
    xp_after   BIGINT    NOT NULL,
    xp_delta   INTEGER   NOT NULL,
    breakdown  JSONB     NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_player_xp_events_user_created
    ON player_xp_events(user_id, created_at DESC);
