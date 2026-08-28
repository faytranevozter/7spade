ALTER TABLE users
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1,
    ADD CONSTRAINT users_version_positive CHECK (version > 0);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM users GROUP BY LOWER(display_name) HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'case-insensitive duplicate display names must be moderated before migration 044';
    END IF;
END $$;

CREATE UNIQUE INDEX users_display_name_unique_ci ON users (LOWER(display_name));
