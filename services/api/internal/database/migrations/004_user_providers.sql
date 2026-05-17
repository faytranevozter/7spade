-- Create user_providers table (multi-provider OAuth per spec)
CREATE TABLE IF NOT EXISTS user_providers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,          -- 'google' | 'github' | 'telegram'
    provider_id TEXT NOT NULL,          -- sub / github numeric id / telegram sub
    email       TEXT,                   -- null for Telegram (not provided)
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE (provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_user_providers_user_id ON user_providers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_providers_provider ON user_providers(provider, provider_id);

-- Allow email to be nullable on users (Telegram users have no email)
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Drop old single-provider columns from users
ALTER TABLE users DROP COLUMN IF EXISTS provider;
ALTER TABLE users DROP COLUMN IF EXISTS provider_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS avatar_url;
