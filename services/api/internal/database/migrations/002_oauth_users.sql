-- Allow password_hash to be NULL for OAuth-only users
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- Add OAuth provider fields
ALTER TABLE users ADD COLUMN IF NOT EXISTS provider VARCHAR(32);
ALTER TABLE users ADD COLUMN IF NOT EXISTS provider_user_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;

-- Allow looking up by provider+provider_user_id and ensure each provider account maps to one user.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_provider_user_id
    ON users(provider, provider_user_id)
    WHERE provider IS NOT NULL AND provider_user_id IS NOT NULL;
