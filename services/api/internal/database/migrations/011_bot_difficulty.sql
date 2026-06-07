ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS bot_difficulty VARCHAR(10) NOT NULL DEFAULT 'medium';

UPDATE rooms SET bot_difficulty = 'medium' WHERE bot_difficulty = 'normal';

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_bot_difficulty_check;

ALTER TABLE rooms
    ADD CONSTRAINT rooms_bot_difficulty_check
    CHECK (bot_difficulty IN ('easy', 'medium', 'hard'));
