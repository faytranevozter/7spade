-- Repair migration: 011_bot_difficulty.sql was edited in place after it had
-- already been applied on some databases, so the constraint/default still used
-- the old 'easy'/'normal'/'hard' vocabulary and rejected 'medium'. Migrations
-- are keyed by filename and never re-run, so this forward migration re-applies
-- the intended 'easy'/'medium'/'hard' contract idempotently.
--
-- Order matters: drop the old CHECK before remapping 'normal' -> 'medium', since
-- the old constraint forbids 'medium' and would reject the UPDATE otherwise.

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS rooms_bot_difficulty_check;

ALTER TABLE rooms
    ALTER COLUMN bot_difficulty SET DEFAULT 'medium';

UPDATE rooms SET bot_difficulty = 'medium' WHERE bot_difficulty = 'normal';

ALTER TABLE rooms
    ADD CONSTRAINT rooms_bot_difficulty_check
    CHECK (bot_difficulty IN ('easy', 'medium', 'hard'));
