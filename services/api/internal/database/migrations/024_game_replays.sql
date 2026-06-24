-- Game replay: per-move log and initial dealt hands for the last 20 games.
-- Older replay data is pruned on each save so storage stays bounded.

-- ============================================================================
-- game_moves — ordered log of every card played in a game
-- ============================================================================
CREATE TABLE IF NOT EXISTS game_moves (
    game_id            UUID         NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    move_index         INT          NOT NULL,
    player_index       SMALLINT     NOT NULL,
    card_rank          SMALLINT,
    card_suit          SMALLINT,
    move_type          VARCHAR(16)  NOT NULL,
    ace_close_direction VARCHAR(4),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, move_index)
);

CREATE INDEX IF NOT EXISTS idx_game_moves_game ON game_moves(game_id, move_index);

-- ============================================================================
-- game_initial_hands — the dealt hand for each seat at game start
-- ============================================================================
CREATE TABLE IF NOT EXISTS game_initial_hands (
    game_id      UUID     NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    player_index SMALLINT NOT NULL,
    hand         JSONB    NOT NULL,
    PRIMARY KEY (game_id, player_index)
);
