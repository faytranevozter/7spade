CREATE TABLE IF NOT EXISTS skin_unlock_rules (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL UNIQUE,
    skin_id        UUID NOT NULL REFERENCES skins(id) ON DELETE CASCADE,
    rule_type      TEXT NOT NULL CHECK (rule_type IN ('achievement')),
    achievement_id TEXT REFERENCES achievements(id) ON DELETE CASCADE,
    enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (rule_type <> 'achievement' OR achievement_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS skin_unlock_rules_achievement_idx
    ON skin_unlock_rules (achievement_id)
    WHERE enabled = TRUE AND rule_type = 'achievement';
