ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS game_mode VARCHAR(20) NOT NULL DEFAULT 'classic',
    ADD COLUMN IF NOT EXISTS max_players INTEGER NOT NULL DEFAULT 4,
    ADD COLUMN IF NOT EXISTS deck_count INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS scoring_mode VARCHAR(20) NOT NULL DEFAULT 'rank_value',
    ADD COLUMN IF NOT EXISTS team_mode VARCHAR(20) NOT NULL DEFAULT 'ffa';

ALTER TABLE rooms
    ADD CONSTRAINT chk_max_players CHECK (max_players BETWEEN 2 AND 8),
    ADD CONSTRAINT chk_deck_count CHECK (deck_count BETWEEN 1 AND 2),
    ADD CONSTRAINT chk_scoring_mode CHECK (scoring_mode IN ('rank_value', 'flat', 'custom')),
    ADD CONSTRAINT chk_team_mode CHECK (team_mode IN ('ffa', '2v2'));

CREATE INDEX IF NOT EXISTS idx_rooms_game_mode ON rooms(game_mode);
