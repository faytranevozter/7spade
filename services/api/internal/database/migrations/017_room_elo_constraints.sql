ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS min_elo INTEGER NULL,
    ADD COLUMN IF NOT EXISTS max_elo INTEGER NULL;

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_elo_range_check;

ALTER TABLE rooms
    ADD CONSTRAINT rooms_elo_range_check
    CHECK (
        (min_elo IS NULL AND max_elo IS NULL)
        OR (min_elo IS NOT NULL AND max_elo IS NOT NULL AND min_elo >= 0 AND max_elo >= min_elo)
    );

CREATE INDEX IF NOT EXISTS idx_rooms_public_waiting_elo
    ON rooms(status, visibility, min_elo, max_elo)
    WHERE practice_mode = false;
