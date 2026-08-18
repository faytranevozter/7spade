CREATE TABLE IF NOT EXISTS skins (
    id            UUID PRIMARY KEY,
    skin_type     TEXT NOT NULL CHECK (skin_type IN ('profile_background', 'avatar_frame', 'display_picture')),
    name          TEXT NOT NULL,
    description   TEXT NOT NULL,
    asset_key     TEXT NOT NULL,
    is_starter    BOOLEAN NOT NULL DEFAULT FALSE,
    display_order INTEGER NOT NULL DEFAULT 0,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_skins (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skin_id   UUID NOT NULL REFERENCES skins(id) ON DELETE CASCADE,
    source    TEXT NOT NULL DEFAULT 'starter',
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, skin_id)
);

CREATE TABLE IF NOT EXISTS user_equipped_skins (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skin_type TEXT NOT NULL CHECK (skin_type IN ('profile_background', 'avatar_frame', 'display_picture')),
    skin_id   UUID NOT NULL REFERENCES skins(id) ON DELETE CASCADE,
    equipped_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, skin_type)
);

INSERT INTO skins (id, skin_type, name, description, asset_key, is_starter, display_order)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'profile_background', 'Gilded Table', 'A warm card-table background for your profile.', 'skins/backgrounds/gilded-table.svg', TRUE, 10),
    ('a0000000-0000-0000-0000-000000000002', 'avatar_frame', 'Gold Spade Frame', 'A polished frame for your avatar.', 'skins/frames/gold-spade.svg', TRUE, 20),
    ('a0000000-0000-0000-0000-000000000003', 'display_picture', 'Ace of Spades', 'A classic Seven Spade display picture.', 'skins/display-pictures/ace-spade.svg', TRUE, 30)
ON CONFLICT (id) DO NOTHING;

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
