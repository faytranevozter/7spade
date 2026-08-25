CREATE TABLE IF NOT EXISTS skin_unlock_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    skin_id           UUID NOT NULL REFERENCES skins(id) ON DELETE CASCADE,
    rule_type         TEXT NOT NULL CHECK (rule_type IN ('achievement', 'game_condition', 'minimum_level', 'login_streak')),
    achievement_id    TEXT REFERENCES achievements(id) ON DELETE CASCADE,
    metric            TEXT,
    operator          TEXT,
    value             TEXT,
    minimum_level     INTEGER,
    login_streak_days INTEGER,
    retroactive       BOOLEAN NOT NULL DEFAULT FALSE,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (rule_type <> 'achievement' OR achievement_id IS NOT NULL),
    CHECK (
        rule_type <> 'game_condition'
        OR (
            metric IN (
                'is_winner', 'shared_win_count', 'penalty', 'games_played', 'wins',
                'current_streak', 'current_top2_streak', 'first_place_count',
                'zero_penalty_games', 'human_only_games', 'all_zero_penalty',
                'ace_closed', 'game_duration_seconds'
            )
            AND operator IN ('eq', 'gte', 'lte', 'gt', 'lt')
            AND value IS NOT NULL
            AND (metric NOT IN ('is_winner', 'all_zero_penalty', 'ace_closed') OR operator = 'eq')
        )
    ),
    CHECK (rule_type <> 'minimum_level' OR (minimum_level IS NOT NULL AND minimum_level >= 1)),
    CHECK (rule_type <> 'login_streak' OR (login_streak_days IS NOT NULL AND login_streak_days >= 1))
);

COMMENT ON COLUMN skin_unlock_rules.retroactive IS
    'Allows deployment reconciliation only when the rule can be reconstructed from retained durable data.';

ALTER TABLE user_skins
    ALTER COLUMN source DROP NOT NULL,
    ALTER COLUMN source DROP DEFAULT,
    ADD COLUMN IF NOT EXISTS skin_unlock_rule_id UUID REFERENCES skin_unlock_rules(id),
    ADD CONSTRAINT user_skins_provenance_check
        CHECK ((source IS NOT NULL) <> (skin_unlock_rule_id IS NOT NULL));

CREATE INDEX IF NOT EXISTS user_skins_unlock_rule_idx
    ON user_skins (skin_unlock_rule_id)
    WHERE skin_unlock_rule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS skin_unlock_rules_achievement_idx
    ON skin_unlock_rules (achievement_id)
    WHERE enabled = TRUE AND rule_type = 'achievement';

CREATE INDEX IF NOT EXISTS skin_unlock_rules_game_condition_idx
    ON skin_unlock_rules (metric)
    WHERE enabled = TRUE AND rule_type = 'game_condition';

CREATE INDEX IF NOT EXISTS skin_unlock_rules_minimum_level_idx
    ON skin_unlock_rules (minimum_level)
    WHERE enabled = TRUE AND rule_type = 'minimum_level';

CREATE INDEX IF NOT EXISTS skin_unlock_rules_login_streak_idx
    ON skin_unlock_rules (login_streak_days)
    WHERE enabled = TRUE AND rule_type = 'login_streak';

CREATE TABLE IF NOT EXISTS user_login_progress (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak  INTEGER NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    best_streak     INTEGER NOT NULL DEFAULT 0 CHECK (best_streak >= current_streak),
    last_login_date DATE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
