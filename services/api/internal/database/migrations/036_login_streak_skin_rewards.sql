CREATE TABLE IF NOT EXISTS user_login_progress (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak  INTEGER NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    best_streak     INTEGER NOT NULL DEFAULT 0 CHECK (best_streak >= current_streak),
    last_login_date DATE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE skin_unlock_rules
    DROP CONSTRAINT IF EXISTS skin_unlock_rules_rule_type_check;

ALTER TABLE skin_unlock_rules
    ADD COLUMN IF NOT EXISTS login_streak_days INTEGER;

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_rule_type_check
    CHECK (rule_type IN ('achievement', 'game_condition', 'minimum_level', 'login_streak'));

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_login_streak_check
    CHECK (
        rule_type <> 'login_streak'
        OR (login_streak_days IS NOT NULL AND login_streak_days >= 1)
    );

CREATE INDEX IF NOT EXISTS skin_unlock_rules_login_streak_idx
    ON skin_unlock_rules (login_streak_days)
    WHERE enabled = TRUE AND rule_type = 'login_streak';

INSERT INTO skins (id, skin_type, name, description, asset_key, is_starter, display_order)
VALUES (
    'a0000000-0000-0000-0000-000000000013',
    'avatar_frame',
    'Seven Day Streak Frame',
    'Awarded for logging in on seven consecutive UTC dates.',
    'skins/frames/gold-spade.svg',
    FALSE,
    130
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO skin_unlock_rules (id, skin_id, rule_type, login_streak_days)
VALUES (
    'login-streak-seven-day-frame',
    'a0000000-0000-0000-0000-000000000013',
    'login_streak',
    7
)
ON CONFLICT (id) DO NOTHING;
