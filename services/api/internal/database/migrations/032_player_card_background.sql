ALTER TABLE skins
    DROP CONSTRAINT IF EXISTS skins_skin_type_check;

ALTER TABLE skins
    DROP CONSTRAINT IF EXISTS skins_type_check;

ALTER TABLE skins
    ADD CONSTRAINT skins_skin_type_check
    CHECK (skin_type IN ('profile_background', 'avatar_frame', 'display_picture', 'player_card_background'));

ALTER TABLE user_equipped_skins
    DROP CONSTRAINT IF EXISTS user_equipped_skins_skin_type_check;

ALTER TABLE user_equipped_skins
    DROP CONSTRAINT IF EXISTS user_equipped_skins_type_check;

ALTER TABLE user_equipped_skins
    ADD CONSTRAINT user_equipped_skins_skin_type_check
    CHECK (skin_type IN ('profile_background', 'avatar_frame', 'display_picture', 'player_card_background'));

INSERT INTO skins (id, skin_type, name, description, asset_key, is_starter, display_order)
VALUES (
    '15fad04f-8866-478d-89fe-a68ac37adabf',
    'player_card_background',
    'Gilded Seat',
    'A gilded felt backdrop for your in-game player card.',
    'skins/player-card-backgrounds/gilded-seat.svg',
    TRUE,
    40
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_skins (user_id, skin_id, source)
SELECT u.id, '15fad04f-8866-478d-89fe-a68ac37adabf', 'starter'
FROM users u
ON CONFLICT (user_id, skin_id) DO NOTHING;
