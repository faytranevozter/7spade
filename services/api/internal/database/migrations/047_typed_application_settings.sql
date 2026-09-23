ALTER TABLE feature_settings
    ADD COLUMN type TEXT,
    ADD COLUMN value JSONB;

UPDATE feature_settings
SET type = 'boolean', value = to_jsonb(enabled);

INSERT INTO feature_settings (key, enabled, type, value) VALUES
    ('daily_login_xp_base', TRUE, 'integer', '10'::jsonb),
    ('daily_login_xp_step', TRUE, 'integer', '5'::jsonb),
    ('daily_login_xp_max', TRUE, 'integer', '50'::jsonb);

ALTER TABLE feature_settings
    ALTER COLUMN type SET NOT NULL,
    ALTER COLUMN value SET NOT NULL,
    DROP COLUMN enabled,
    ADD CONSTRAINT feature_settings_type_valid CHECK (
        type IN ('boolean', 'integer', 'float', 'string', 'options')
    ),
    ADD CONSTRAINT feature_settings_value_matches_type CHECK (
        (type = 'boolean' AND jsonb_typeof(value) = 'boolean')
        OR (type = 'integer' AND jsonb_typeof(value) = 'number' AND value::text ~ '^-?[0-9]+$')
        OR (type = 'float' AND jsonb_typeof(value) = 'number')
        OR (type = 'string' AND jsonb_typeof(value) = 'string')
        OR (type = 'options' AND jsonb_typeof(value) IN ('array', 'object'))
    );
