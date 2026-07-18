-- Account deletion: schedule → 7-day grace → hard delete + seat anonymization.
ALTER TABLE users ADD COLUMN IF NOT EXISTS deletion_scheduled_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_deletion_scheduled_at
    ON users (deletion_scheduled_at)
    WHERE deletion_scheduled_at IS NOT NULL;
