ALTER TABLE users
    ADD COLUMN suspended_at TIMESTAMPTZ,
    ADD COLUMN suspension_expires_at TIMESTAMPTZ,
    ADD COLUMN suspension_reason TEXT,
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1,
    ADD CONSTRAINT users_version_positive CHECK (version > 0);

CREATE INDEX users_active_suspension_idx ON users (suspension_expires_at)
    WHERE suspended_at IS NOT NULL;
