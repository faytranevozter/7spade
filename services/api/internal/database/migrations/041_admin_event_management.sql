ALTER TABLE events
    ADD COLUMN lifecycle_state TEXT NOT NULL DEFAULT 'draft' CHECK (lifecycle_state IN ('draft', 'scheduled', 'published', 'archived')),
    ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    ADD COLUMN resource_version INTEGER NOT NULL DEFAULT 1 CHECK (resource_version > 0),
    ADD COLUMN reward_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN published_at TIMESTAMPTZ,
    ADD COLUMN archived_at TIMESTAMPTZ;

UPDATE events
SET lifecycle_state = CASE WHEN enabled THEN 'published' ELSE 'draft' END,
    published_at = CASE WHEN enabled THEN updated_at ELSE NULL END;

CREATE TABLE event_versions (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE RESTRICT,
    revision INTEGER NOT NULL CHECK (revision > 0),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    summary TEXT NOT NULL,
    description TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    hero_asset_key TEXT,
    accent_color TEXT,
    reward_config JSONB NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event_id, revision),
    CHECK (starts_at < ends_at)
);

INSERT INTO event_versions(event_id, revision, slug, name, summary, description, starts_at, ends_at, hero_asset_key, accent_color, reward_config, published_at)
SELECT id, 1, slug, name, summary, description, starts_at, ends_at, hero_asset_key, accent_color, reward_config, COALESCE(published_at, updated_at)
FROM events
WHERE enabled
ON CONFLICT DO NOTHING;

ALTER TABLE event_check_ins ADD COLUMN event_revision INTEGER;
UPDATE event_check_ins SET event_revision = 1 WHERE event_revision IS NULL;
ALTER TABLE event_check_ins ALTER COLUMN event_revision SET NOT NULL;
ALTER TABLE event_check_ins ALTER COLUMN event_revision SET DEFAULT 1;
ALTER TABLE event_check_ins ADD CONSTRAINT event_check_ins_version_fk FOREIGN KEY (event_id, event_revision) REFERENCES event_versions(event_id, revision) NOT VALID;

ALTER TABLE user_skins ADD COLUMN event_id UUID REFERENCES events(id) ON DELETE RESTRICT;
ALTER TABLE user_skins ADD COLUMN event_revision INTEGER;
ALTER TABLE user_skins ADD CONSTRAINT user_skins_event_version_fk FOREIGN KEY (event_id, event_revision) REFERENCES event_versions(event_id, revision) NOT VALID;
ALTER TABLE user_skins ADD CONSTRAINT user_skins_event_version_pair CHECK ((event_id IS NULL) = (event_revision IS NULL));
