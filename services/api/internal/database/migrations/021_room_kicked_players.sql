-- Records players a host has kicked from a room so they cannot rejoin it. The
-- WS server has an in-memory block, but the HTTP join path persists a
-- membership row before the socket connects, so the block must live in the DB
-- too. Rows are cleared when the room is deleted (ON DELETE CASCADE).
CREATE TABLE IF NOT EXISTS room_kicked_players (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    kicked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, user_id)
);
