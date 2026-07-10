-- Retained post-game result details for the latest finished games.
-- These rows intentionally start only after this migration; older games keep
-- their basic history rows but do not expose the detailed historical results UI.

CREATE TABLE IF NOT EXISTS game_result_details (
    game_id         UUID        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    player_index    INTEGER     NOT NULL,
    subject_id      TEXT,
    is_guest        BOOLEAN     NOT NULL DEFAULT FALSE,
    team            SMALLINT,
    face_down_cards JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, player_index)
);

CREATE INDEX IF NOT EXISTS idx_game_result_details_game ON game_result_details(game_id, player_index);
