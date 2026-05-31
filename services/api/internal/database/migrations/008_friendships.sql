-- Friendships: one directed row per request (requester -> addressee). An
-- accepted friendship is queried in both directions. status is one of
-- 'pending' | 'accepted' | 'blocked'. A composite PK prevents duplicate
-- directed rows; the CHECK forbids self-friendship.
CREATE TABLE IF NOT EXISTS friendships (
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (requester_id, addressee_id),
    CHECK (requester_id <> addressee_id),
    CHECK (status IN ('pending', 'accepted', 'blocked'))
);

-- Reverse-direction lookups (incoming requests, "is this user my friend").
CREATE INDEX IF NOT EXISTS idx_friendships_addressee ON friendships(addressee_id);
