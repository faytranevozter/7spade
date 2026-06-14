-- Denormalize the room name onto saved games so history can show a friendly
-- label. The rooms row may be deleted after a game ends, so we copy the name in
-- at save time rather than joining at read time. Backfill existing games from
-- rooms where the row still exists.
ALTER TABLE games
    ADD COLUMN IF NOT EXISTS room_name VARCHAR(60) NULL;

UPDATE games g
SET room_name = r.name
FROM rooms r
WHERE g.room_name IS NULL
  AND g.room_id ~ '^[0-9a-fA-F-]{36}$'
  AND r.id = g.room_id::uuid;
