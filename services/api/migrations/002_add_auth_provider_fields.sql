ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(32) NOT NULL DEFAULT 'password',
    ADD COLUMN IF NOT EXISTS provider_id VARCHAR(255);

UPDATE users SET provider_id = email WHERE provider_id IS NULL;

ALTER TABLE users
    ALTER COLUMN provider_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_auth_provider_provider_id ON users(auth_provider, provider_id);
