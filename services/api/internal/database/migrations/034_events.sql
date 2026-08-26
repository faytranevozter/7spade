CREATE TABLE IF NOT EXISTS events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    summary        TEXT NOT NULL DEFAULT '',
    description    TEXT NOT NULL DEFAULT '',
    starts_at      TIMESTAMPTZ NOT NULL,
    ends_at        TIMESTAMPTZ NOT NULL,
    timezone       TEXT NOT NULL DEFAULT 'UTC',
    hero_asset_key TEXT,
    accent_color   TEXT,
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (slug <> ''),
    CHECK (starts_at < ends_at)
);

CREATE TABLE IF NOT EXISTS event_check_ins (
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_day  DATE NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, user_id, event_day)
);

ALTER TABLE skin_unlock_rules
    DROP CONSTRAINT IF EXISTS skin_unlock_rules_rule_type_check,
    ADD COLUMN IF NOT EXISTS event_id UUID REFERENCES events(id),
    ADD COLUMN IF NOT EXISTS event_check_in_count INTEGER,
    ADD CONSTRAINT skin_unlock_rules_rule_type_check CHECK (
        rule_type IN ('achievement', 'game_condition', 'minimum_level', 'login_streak', 'event_check_in_count')
    ),
    ADD CONSTRAINT skin_unlock_rules_event_check_in_check CHECK (
        (rule_type = 'event_check_in_count'
            AND event_id IS NOT NULL
            AND event_check_in_count IS NOT NULL
            AND event_check_in_count >= 1
            AND achievement_id IS NULL
            AND minimum_level IS NULL
            AND login_streak_days IS NULL)
        OR (rule_type <> 'event_check_in_count' AND event_check_in_count IS NULL)
    );

CREATE INDEX IF NOT EXISTS events_active_idx ON events (starts_at, ends_at) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS skin_unlock_rules_event_idx ON skin_unlock_rules (event_id) WHERE event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS event_check_ins_user_idx ON event_check_ins (user_id, event_id);
