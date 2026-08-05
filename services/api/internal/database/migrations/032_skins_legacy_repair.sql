DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'skins' AND column_name = 'type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'skins' AND column_name = 'skin_type'
    ) THEN
        ALTER TABLE skins RENAME COLUMN type TO skin_type;
    END IF;
END;
$$;

ALTER TABLE skins ADD COLUMN IF NOT EXISTS is_starter BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_skins ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'starter';
ALTER TABLE user_equipped_skins ADD COLUMN IF NOT EXISTS equipped_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

INSERT INTO skins (id, skin_type, name, description, asset_key, is_starter, display_order, enabled)
VALUES
    ('background_gilded_table', 'profile_background', 'Gilded Table', 'A warm card-table background for your profile.', 'skins/backgrounds/gilded-table.svg', TRUE, 10, TRUE),
    ('frame_gold_spade', 'avatar_frame', 'Gold Spade Frame', 'A polished frame for your avatar.', 'skins/frames/gold-spade.svg', TRUE, 20, TRUE),
    ('picture_ace_spade', 'display_picture', 'Ace of Spades', 'A classic Seven Spade display picture.', 'skins/display-pictures/ace-spade.svg', TRUE, 30, TRUE)
ON CONFLICT (id) DO UPDATE
SET skin_type = EXCLUDED.skin_type,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    asset_key = EXCLUDED.asset_key,
    is_starter = EXCLUDED.is_starter,
    display_order = EXCLUDED.display_order,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

INSERT INTO user_skins (user_id, skin_id, source)
SELECT u.id, s.id, 'starter'
FROM users u
CROSS JOIN skins s
WHERE s.is_starter = TRUE
ON CONFLICT (user_id, skin_id) DO NOTHING;

CREATE OR REPLACE FUNCTION grant_starter_skins()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_skins (user_id, skin_id, source)
    SELECT NEW.id, s.id, 'starter'
    FROM skins s
    WHERE s.is_starter = TRUE
    ON CONFLICT (user_id, skin_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_grant_starter_skins ON users;
CREATE TRIGGER users_grant_starter_skins
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION grant_starter_skins();
