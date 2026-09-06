UPDATE events
SET reward_config = jsonb_set(
    COALESCE(reward_config, '{}'::jsonb),
    '{daily_login}',
    '{"enabled": true, "xp_per_claim": 100}'::jsonb,
    true
)
WHERE NOT (COALESCE(reward_config, '{}'::jsonb) ? 'daily_login');

CREATE TABLE IF NOT EXISTS event_check_in_xp_events (
    event_id       UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    event_revision INTEGER NOT NULL,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    claim_date     DATE NOT NULL,
    xp_before      BIGINT NOT NULL,
    xp_after       BIGINT NOT NULL,
    xp_delta       INTEGER NOT NULL CHECK (xp_delta > 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, user_id, claim_date),
    CONSTRAINT event_check_in_xp_version_fk
        FOREIGN KEY (event_id, event_revision)
        REFERENCES event_versions(event_id, revision)
);

CREATE INDEX IF NOT EXISTS event_check_in_xp_events_user_created_idx
    ON event_check_in_xp_events (user_id, created_at DESC);
