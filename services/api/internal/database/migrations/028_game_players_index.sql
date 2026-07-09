-- Repoint the game_players primary key from (game_id, display_name) to
-- (game_id, player_index). Two seated players can legitimately share a
-- display name (e.g. two guests named "Guest"), which made the old key
-- reject an otherwise valid game save. player_index is a stable seat (0..3)
-- carried by the WS service, matching the replay move/initial-hand indexing.

ALTER TABLE game_players ADD COLUMN player_index INTEGER NOT NULL DEFAULT 0;

-- Backfill historical rows with a stable-ish seat ordering by rank. Existing
-- games only need a unique (game_id, player_index) value; correctness of the
-- seat index matters only for rows written after this migration.
UPDATE game_players
SET player_index = sub.seat
FROM (
    SELECT game_id, display_name, rank,
           ROW_NUMBER() OVER (PARTITION BY game_id ORDER BY rank ASC) - 1 AS seat
    FROM game_players
) sub
WHERE game_players.game_id = sub.game_id
  AND game_players.display_name = sub.display_name
  AND game_players.rank = sub.rank;

ALTER TABLE game_players DROP CONSTRAINT game_players_pkey;
ALTER TABLE game_players ADD PRIMARY KEY (game_id, player_index);
