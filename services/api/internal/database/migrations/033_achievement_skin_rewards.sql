CREATE TABLE IF NOT EXISTS skin_unlock_rules (
    id             TEXT PRIMARY KEY,
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

INSERT INTO skins (id, skin_type, name, description, asset_key, is_starter, display_order)
VALUES (
    '22c398f1-a7fc-42f1-b53f-a1a394cf8d37',
    'avatar_frame',
    'First Victory Frame',
    'A celebratory frame awarded for earning First Win.',
    'skins/frames/first-victory.svg',
    FALSE,
    100
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO skin_unlock_rules (id, skin_id, rule_type, achievement_id)
VALUES (
    'achievement-first-win-frame',
    '22c398f1-a7fc-42f1-b53f-a1a394cf8d37',
    'achievement',
    'first_win'
)
ON CONFLICT (id) DO NOTHING;
