CREATE TABLE IF NOT EXISTS games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    finished_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS game_players (
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    display_name VARCHAR(50) NOT NULL,
    penalty_points INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    is_winner BOOLEAN NOT NULL,
    PRIMARY KEY (game_id, display_name)
);

CREATE INDEX IF NOT EXISTS idx_games_finished_at ON games(finished_at DESC);
CREATE INDEX IF NOT EXISTS idx_game_players_user_id ON game_players(user_id);
