ALTER TABLE skin_unlock_rules
    DROP CONSTRAINT IF EXISTS skin_unlock_rules_rule_type_check;

ALTER TABLE skin_unlock_rules
    ADD COLUMN IF NOT EXISTS minimum_level INTEGER;

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_rule_type_check
    CHECK (rule_type IN ('achievement', 'game_condition', 'minimum_level'));

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_minimum_level_check
    CHECK (
        rule_type <> 'minimum_level'
        OR (minimum_level IS NOT NULL AND minimum_level >= 1)
    );

CREATE INDEX IF NOT EXISTS skin_unlock_rules_minimum_level_idx
    ON skin_unlock_rules (minimum_level)
    WHERE enabled = TRUE AND rule_type = 'minimum_level';
