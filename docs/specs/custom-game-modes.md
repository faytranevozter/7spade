# Custom Game Modes

Custom game modes allow players to create rooms with non-standard rules. Custom
games are casual-only (no ELO impact) and are accessed via the dedicated
"Custom Game" button in the lobby.

## Configuration Options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `game_mode` | `classic`, `custom` | `classic` | Must be `custom` to enable the options below. |
| `max_players` | 2–8 | 4 | Number of seats. Bots fill remaining seats at game start. |
| `deck_count` | 1, 2 | 1 | 1 = standard 52 cards, 2 = double deck (104 cards). |
| `scoring_mode` | `rank_value`, `flat`, `custom` | `rank_value` | How face-down penalty cards are scored. |
| `custom_scores` | `{ rank: points }` | — | Per-rank penalty override. Only used with `scoring_mode: "custom"`. Ranks are integers 2–14 (14 = Ace). |
| `team_mode` | `ffa`, `2v2` | `ffa` | Free-for-all or 2v2 teams. `2v2` requires exactly 4 players. |

## Scoring Modes

### `rank_value` (Classic)

Each face-down card's penalty equals its rank value (2–13). Aces score 1 (if
closed low), 14 (if closed high), or 7 (if no ace was closed during the game).

### `flat`

Every face-down card = 1 penalty point regardless of rank.

### `custom`

The room creator assigns a penalty value per rank. Unspecified ranks fall back
to `rank_value` scoring. The `custom_scores` map is stored as JSONB and flows
through the full pipeline: API → WS → game engine.

## Double Deck (104 Cards)

When `deck_count` is 2, the deck contains two copies of each card. Key rules:

- **Dealing:** 104 cards divided among players (e.g., 26 each for 4 players).
- **Stacking:** A duplicate card whose rank is already on the board for that
  suit is playable — it "stacks" on the existing position without extending
  the sequence.
- **Board display:** Stacked positions show a count badge (e.g., "2") in the UI.
- **Bot inference:** The bot's unknown-card universe accounts for 104 cards.

## Team Mode (2v2)

When `team_mode` is `"2v2"`:

- Exactly 4 players, 2 per team.
- Players choose their team in the waiting room via `set_team` messages.
- Each team is capped at 2 members; the server rejects joins to a full team.
- Bots backfill to balance teams.
- **Shared hand visibility:** Teammates can see each other's full hand during
  gameplay (sent in the `opponents` payload as `hand`).
- **Team scoring:** Each player's penalty is the sum of both teammates'
  face-down cards. The team with the lowest combined penalty wins.
- **Results:** The `game_over` message includes `team` per player and the
  results screen groups players by team.

## Database Schema

Migration `026_custom_game_modes.sql`:
```sql
ALTER TABLE rooms
    ADD COLUMN game_mode VARCHAR(20) NOT NULL DEFAULT 'classic',
    ADD COLUMN max_players INTEGER NOT NULL DEFAULT 4,
    ADD COLUMN deck_count INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN scoring_mode VARCHAR(20) NOT NULL DEFAULT 'rank_value',
    ADD COLUMN team_mode VARCHAR(20) NOT NULL DEFAULT 'ffa';
```

Migration `027_custom_scores.sql`:
```sql
ALTER TABLE rooms ADD COLUMN custom_scores JSONB;
```

## Engine Architecture

The `GameConfig` struct travels with the game state:

```go
type GameConfig struct {
    PlayerCount  int
    DeckCount    int
    ScoringMode  ScoringMode    // "rank_value", "flat", "custom"
    CustomScores map[Rank]int
    TeamMode     TeamMode       // "ffa", "2v2"
    Teams        [][]int
    StartingSuit Suit
}
```

It is:
- Created from `roomSettings` when a room is first joined
- Passed to `DealWithConfig()` at game start
- Stored in the Redis room snapshot for rehydration
- Used by `CalculateScores()` and bot strategies
