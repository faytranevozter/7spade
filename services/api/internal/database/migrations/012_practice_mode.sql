ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS practice_mode BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_rooms_practice_mode ON rooms(practice_mode);
