ALTER TABLE skin_unlock_rules
    ADD CONSTRAINT skin_unlock_rules_id_event_unique UNIQUE (id, event_id);

CREATE TABLE skin_unlock_rule_event_versions (
    skin_unlock_rule_id UUID NOT NULL,
    event_id UUID NOT NULL,
    event_revision INTEGER NOT NULL,
    associated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (skin_unlock_rule_id, event_id, event_revision),
    CONSTRAINT skin_unlock_rule_event_versions_rule_fk
        FOREIGN KEY (skin_unlock_rule_id, event_id)
        REFERENCES skin_unlock_rules(id, event_id)
        ON DELETE CASCADE,
    CONSTRAINT skin_unlock_rule_event_versions_event_fk
        FOREIGN KEY (event_id, event_revision)
        REFERENCES event_versions(event_id, revision)
);

INSERT INTO skin_unlock_rule_event_versions (skin_unlock_rule_id, event_id, event_revision)
SELECT id, event_id, event_revision
FROM skin_unlock_rules
WHERE event_id IS NOT NULL AND event_revision IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE INDEX skin_unlock_rule_event_versions_lookup_idx
    ON skin_unlock_rule_event_versions (skin_unlock_rule_id, event_id, event_revision DESC);
