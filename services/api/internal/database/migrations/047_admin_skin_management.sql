CREATE TABLE skin_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    skin_id UUID NOT NULL REFERENCES skins(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version > 0),
    asset_key TEXT NOT NULL,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/png','image/jpeg','image/webp','image/svg+xml')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    published_by UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled_by UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    disabled_at TIMESTAMPTZ,
    UNIQUE (skin_id, version), UNIQUE (skin_id, asset_key), UNIQUE (id, skin_id),
    CHECK ((disabled_at IS NULL) = enabled)
);

CREATE OR REPLACE FUNCTION reject_skin_revision_mutation() RETURNS TRIGGER AS $$
BEGIN
    IF OLD.id <> NEW.id OR OLD.skin_id <> NEW.skin_id OR OLD.version <> NEW.version OR OLD.asset_key <> NEW.asset_key OR OLD.content_type <> NEW.content_type OR OLD.published_by IS DISTINCT FROM NEW.published_by OR OLD.published_at <> NEW.published_at THEN
        RAISE EXCEPTION 'published skin revisions are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER skin_revisions_immutable BEFORE UPDATE ON skin_revisions FOR EACH ROW EXECUTE FUNCTION reject_skin_revision_mutation();

INSERT INTO skin_revisions (skin_id, version, asset_key, content_type)
SELECT id, 1, asset_key,
       CASE lower(substring(asset_key from '\.[^.]+$'))
           WHEN '.jpg' THEN 'image/jpeg' WHEN '.jpeg' THEN 'image/jpeg'
            WHEN '.webp' THEN 'image/webp' WHEN '.svg' THEN 'image/svg+xml' ELSE 'image/png'
       END
FROM skins
WHERE asset_key <> '';

ALTER TABLE user_skins ADD COLUMN skin_revision_id UUID;
UPDATE user_skins us SET skin_revision_id = sr.id
FROM skin_revisions sr WHERE sr.skin_id = us.skin_id AND sr.version = 1;

CREATE OR REPLACE FUNCTION pin_user_skin_revision() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.skin_revision_id IS NULL THEN
        SELECT sr.id INTO NEW.skin_revision_id
        FROM skins s
        JOIN skin_revisions sr ON sr.skin_id = s.id AND sr.asset_key = s.asset_key
        WHERE s.id = NEW.skin_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER user_skins_pin_revision BEFORE INSERT ON user_skins FOR EACH ROW EXECUTE FUNCTION pin_user_skin_revision();

ALTER TABLE user_skins
    ALTER COLUMN skin_revision_id SET NOT NULL,
    ADD CONSTRAINT user_skins_skin_revision_fk
        FOREIGN KEY (skin_revision_id, skin_id) REFERENCES skin_revisions(id, skin_id) ON DELETE RESTRICT;

CREATE TABLE user_skin_entitlement_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skin_id UUID NOT NULL REFERENCES skins(id) ON DELETE RESTRICT,
    skin_revision_id UUID NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('grant','revoke')),
    reason TEXT NOT NULL CHECK (length(btrim(reason)) > 0),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_skin_entitlement_events_skin_revision_fk
        FOREIGN KEY (skin_revision_id, skin_id) REFERENCES skin_revisions(id, skin_id) ON DELETE RESTRICT
);
CREATE INDEX user_skin_entitlement_events_user_idx ON user_skin_entitlement_events(user_id, occurred_at DESC);

INSERT INTO admin_permissions(name,description) VALUES ('skins.entitlements','Grant and revoke exceptional skin entitlements') ON CONFLICT DO NOTHING;
INSERT INTO admin_role_permissions(role_id,permission_name) SELECT id,'skins.entitlements' FROM admin_roles WHERE name='super_admin' ON CONFLICT DO NOTHING;
