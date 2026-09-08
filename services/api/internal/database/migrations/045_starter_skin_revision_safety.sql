CREATE OR REPLACE FUNCTION grant_starter_skins()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_skins (user_id, skin_id, source, skin_revision_id)
    SELECT NEW.id, s.id, 'starter', sr.id
    FROM skins s
    JOIN skin_revisions sr
      ON sr.skin_id = s.id AND sr.asset_key = s.asset_key AND sr.enabled
    WHERE s.is_starter = TRUE AND s.enabled = TRUE
    ON CONFLICT (user_id, skin_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
