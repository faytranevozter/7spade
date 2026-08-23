ALTER TABLE skin_unlock_rules
    ADD COLUMN IF NOT EXISTS retroactive BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN skin_unlock_rules.retroactive IS
    'Allows deployment reconciliation only when the rule can be reconstructed from retained durable data.';
