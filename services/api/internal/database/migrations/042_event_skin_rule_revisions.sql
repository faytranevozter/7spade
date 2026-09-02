ALTER TABLE skin_unlock_rules
    ADD COLUMN event_revision INTEGER;

UPDATE skin_unlock_rules r
SET event_revision = ev.revision
FROM events e
JOIN event_versions ev ON ev.event_id = e.id AND ev.revision = e.revision
WHERE e.id = r.event_id;

ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_event_version_pair
        CHECK (event_id IS NOT NULL OR event_revision IS NULL),
    ADD CONSTRAINT skin_unlock_rules_event_version_fk
        FOREIGN KEY (event_id, event_revision)
        REFERENCES event_versions(event_id, revision);
