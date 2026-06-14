-- Add a per-room number and a display name. room_number is a globally
-- increasing sequence used to build the default name "Room #<n>"; existing
-- rooms get numbers from the BIGSERIAL backfill. The name column is nullable
-- so CreateRoom can COALESCE a blank name to the default referencing the
-- generated room_number.
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS room_number BIGSERIAL;

ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS name VARCHAR(60) NULL;

-- Backfill names for any pre-existing rooms.
UPDATE rooms SET name = 'Room #' || room_number WHERE name IS NULL;
