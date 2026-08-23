ALTER TABLE skin_unlock_rules
    DROP CONSTRAINT IF EXISTS skin_unlock_rules_rule_type_check;

ALTER TABLE skin_unlock_rules
    ADD COLUMN IF NOT EXISTS metric TEXT,
    ADD COLUMN IF NOT EXISTS operator TEXT,
    ADD COLUMN IF NOT EXISTS value TEXT;

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_rule_type_check
    CHECK (rule_type IN ('achievement', 'game_condition'));

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_game_condition_fields_check
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
    );

CREATE INDEX IF NOT EXISTS skin_unlock_rules_game_condition_idx
    ON skin_unlock_rules (metric)
    WHERE enabled = TRUE AND rule_type = 'game_condition';
