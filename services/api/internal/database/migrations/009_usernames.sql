-- Usernames: a lowercase, unique handle used to add friends. Display names stay
-- non-unique and human-facing; usernames are the stable lookup key. Stored
-- normalized (lowercase, [a-z0-9_], 3-32 chars).
ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(32);

-- Backfill existing rows (all users rows are registered/OAuth; guests have no
-- row). Derive a base from the display name: lowercase, non-[a-z0-9_] runs
-- collapsed to '_', trimmed, truncated, falling back to 'player'. Each row is
-- assigned by probing the live table for the next free candidate, so the result
-- is globally unique *before* the unique index is created. A plain
-- ROW_NUMBER-per-base scheme is not enough: a suffixed value like 'alice_2' can
-- collide with a different user whose display name normalizes to 'alice_2', and
-- that cross-partition collision would fail the unique index and abort startup.
DO $$
DECLARE
    r         RECORD;
    base      TEXT;
    candidate TEXT;
    n         INT;
BEGIN
    FOR r IN
        SELECT id, display_name FROM users WHERE username IS NULL ORDER BY created_at, id
    LOOP
        base := NULLIF(
            btrim(regexp_replace(lower(coalesce(r.display_name, '')), '[^a-z0-9_]+', '_', 'g'), '_'),
            ''
        );
        IF base IS NULL OR char_length(base) < 3 THEN
            base := 'player';
        END IF;
        -- Leave headroom (24 chars) for a '_<n>' suffix within VARCHAR(32).
        base := btrim(left(base, 24), '_');
        IF char_length(base) < 3 THEN
            base := 'player';
        END IF;

        candidate := base;
        n := 1;
        WHILE EXISTS (SELECT 1 FROM users WHERE username = candidate) LOOP
            n := n + 1;
            candidate := base || '_' || n;
        END LOOP;

        UPDATE users SET username = candidate WHERE id = r.id;
    END LOOP;
END $$;

ALTER TABLE users ALTER COLUMN username SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
