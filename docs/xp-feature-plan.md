# XP Feature Plan

## Goals

Add lifetime experience points (XP) and level progression for registered players. XP should reward completed non-practice games, be auditable per game, appear on profiles, and support leaderboard sorting.

## Product decisions

- XP is lifetime-only and does not reset by season.
- Player level is derived from total XP.
- Store aggregate XP in `user_stats`.
- Store per-game XP audit records in `player_xp_events`.
- Bot-mixed games award reduced XP.
- Practice games do not award XP.
- Guests and bots do not receive XP.
- XP and level appear on profile pages.
- Leaderboard supports sorting by XP.

## XP formula

Initial formula:

| Source | XP |
| --- | ---: |
| Completed saved game | 50 |
| Rank 1 bonus | 50 |
| Rank 2 bonus | 30 |
| Rank 3 bonus | 15 |
| Rank 4 bonus | 0 |
| Winner bonus | 25 |
| Zero penalty bonus | 25 |
| Low penalty bonus, `penalty_points <= 10` | 10 |
| Human-only bonus | 20 |

Bot-mixed games use a `0.6` multiplier after bonuses.

Minimum XP for a saved registered-player game: `20`.

Practice games are excluded because the existing history, stats, rating, and achievement pipeline already excludes practice mode.

## Level curve

Level is derived from lifetime XP and does not need to be stored.

```text
level = floor(sqrt(total_xp / 100)) + 1
xp_required_for_level(n) = (n - 1)^2 * 100
```

Examples:

| Level | Required total XP |
| ---: | ---: |
| 1 | 0 |
| 2 | 100 |
| 3 | 400 |
| 4 | 900 |
| 5 | 1600 |
| 10 | 8100 |

Stats responses should expose enough information for progress bars:

```json
{
  "xp": 1250,
  "level": 4,
  "xp_into_level": 350,
  "xp_for_next_level": 700,
  "xp_to_next_level": 350
}
```

## Backend plan

### Database migration

Create a new migration under `services/api/internal/database/migrations/`. The latest migration is `022_more_achievements.sql`, so name the new file `023_player_xp.sql`.

Add aggregate XP:

```sql
ALTER TABLE user_stats
ADD COLUMN xp BIGINT NOT NULL DEFAULT 0;

CREATE INDEX idx_user_stats_xp ON user_stats(xp DESC);
```

Create XP event table:

```sql
CREATE TABLE player_xp_events (
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    xp_before BIGINT NOT NULL,
    xp_after BIGINT NOT NULL,
    xp_delta INTEGER NOT NULL,
    breakdown JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, user_id)
);

CREATE INDEX idx_player_xp_events_user_created
ON player_xp_events(user_id, created_at DESC);
```

`breakdown` should contain the formula inputs, for example:

```json
{
  "base": 50,
  "rank_bonus": 50,
  "winner_bonus": 25,
  "zero_penalty_bonus": 0,
  "low_penalty_bonus": 10,
  "human_only_bonus": 20,
  "bot_mixed_multiplier": 1.0,
  "minimum_applied": false
}
```

### XP domain helper

Add `services/api/internal/repository/xp.go`.

Responsibilities:

- Calculate XP delta from one saved game player result.
- Calculate level from XP.
- Calculate XP required for a level.
- Calculate progress within the current level.

Suggested functions:

```go
type XPBreakdown struct {
    Base              int     `json:"base"`
    RankBonus         int     `json:"rank_bonus"`
    WinnerBonus       int     `json:"winner_bonus"`
    ZeroPenaltyBonus  int     `json:"zero_penalty_bonus"`
    LowPenaltyBonus   int     `json:"low_penalty_bonus"`
    HumanOnlyBonus    int     `json:"human_only_bonus"`
    BotMixedMultiplier float64 `json:"bot_mixed_multiplier"`
    MinimumApplied    bool    `json:"minimum_applied"`
}

func CalculateXP(player GameResultPlayer, hasBot bool) (int, XPBreakdown)
func LevelFromXP(xp int64) int
func XPRequiredForLevel(level int) int64
func XPProgress(xp int64) (level int, intoLevel int64, forNextLevel int64, toNextLevel int64)
```

Use existing result fields from `SaveGame`:

- `Rank`
- `IsWinner`
- `PenaltyPoints`
- game-level `hasBot`

### Integrate into game save pipeline

Update `services/api/internal/repository/game.go` inside `SaveGame`.

The integration point is the existing per-player loop, specifically the `if userID != nil` block (game.go:174-217) that already calls `UpsertUserStats`, mirrors season stats, queues ELO, and awards achievements. XP fits alongside this work, inside the same transaction.

Flow per registered player:

1. Calculate XP delta and breakdown via `CalculateXP(player, hasBot)`.
2. Award XP to `user_stats` and read back the new total so `xp_before`/`xp_after` stay self-consistent (same pattern as `insertRatingEvent`, game.go:310-324).
3. Insert `player_xp_events` with `xp_before`, `xp_after`, `xp_delta`, and `breakdown`.

Preferred implementation for step 2: fold the XP increment into the existing `UpsertUserStats` upsert rather than issuing a separate read + UPDATE. `UpsertUserStats` already inserts/updates the `user_stats` row once per registered player, so adding an `xp` increment there avoids a second round-trip and keeps the value consistent within the transaction.

- Extend `UpsertUserStatsParams` with `XPDelta int`.
- Add `xp` to the INSERT column list (initial value `$N`) and the `ON CONFLICT DO UPDATE SET xp = user_stats.xp + $N`.
- Add `xp` to the `RETURNING` clause and to `StatsSnapshot` so the caller gets `xp_after` without a re-read.
- Compute `xp_before = xp_after - delta` when writing the event row.

Note that `UpsertSeasonUserStats` shares `UpsertUserStatsParams`. Because XP is lifetime-only, the season upsert must ignore `XPDelta` (do not add an `xp` column to `season_user_stats`).

Important rules:

- Only registered users receive XP (the `if userID != nil` guard already enforces this; guests and bots have a nil `user_id`).
- Practice games are already skipped before `POST /internal/games`, so no extra practice handling should be necessary in `SaveGame`.
- XP must be awarded even if rating does not change (e.g. a lone human among bots, where `len(eloPlayers) < 2`).
- XP must be awarded regardless of shared wins.

### Stats DTOs and queries

Update `services/api/internal/repository/stats.go`.

Add fields to user stats DTO (`UserStats`):

```go
XP             int64 `json:"xp"`
Level          int   `json:"level"`
XPIntoLevel    int64 `json:"xp_into_level"`
XPForNextLevel int64 `json:"xp_for_next_level"`
XPToNextLevel  int64 `json:"xp_to_next_level"`
```

Add fields to leaderboard DTO (`LeaderboardEntry`):

```go
XP    int64 `json:"xp"`
Level int   `json:"level"`
```

Supporting changes in the same file:

- Add `XP int64` to `StatsSnapshot` so `SaveGame` reads back the new total from the `RETURNING` clause.
- Add `XPDelta int` to `UpsertUserStatsParams`, and wire it into the `UpsertUserStats` INSERT / ON CONFLICT / RETURNING (see the SaveGame section above).
- In `GetUserStats`, select `s.xp` from `user_stats` and derive `Level`, `XPIntoLevel`, `XPForNextLevel`, `XPToNextLevel` via the XP helper. Note `GetUserStats` reads either table depending on season scope; the season path has no `xp` column, so default season XP to `0`/level `1` (or omit progression in the season view).

### Leaderboard sorting

Add sort key:

```text
xp
```

Add an entry to the `leaderboardOrders` allowlist in `services/api/internal/repository/stats.go` (around stats.go:230):

```go
"xp": `
	ORDER BY xp DESC,
	         games_played DESC,
	         user_id ASC
`,
```

Seasonal leaderboard hazard: `GetLeaderboard` reuses the same ORDER BY fragment for both the all-time table (`user_stats`) and the season table (`season_user_stats`) (stats.go:331-353). Because XP lives only in `user_stats`, a `sort=xp` request scoped to a season would reference a non-existent `xp` column and fail with a SQL error.

This makes the guard mandatory, not optional. The leaderboard handler must reject `sort=xp` when a `season` is provided (return a validation error or coerce the sort back to the default). Do not silently sort a season query by `xp`.

Also expose XP/level in the leaderboard rows: add `s.xp` to the `SELECT` list and scan it into the new `LeaderboardEntry.XP` field, then derive `Level` in Go. This only applies to the all-time path; the season path does not select `xp`.

### Optional XP history endpoint

Can be implemented later.

Endpoint:

```text
GET /users/:id/xp-history?page=1&per_page=20
```

Returns rows from `player_xp_events`.

This is not required for the first version because profile and leaderboard only need aggregate XP.

## Web plan

Update `web/src/api/stats.ts`:

- Add XP fields to `UserStatsDto`.
- Add XP/level fields to `LeaderboardEntryDto`.
- Add `xp` to leaderboard sort options.

Update profile UI:

- `web/src/pages/ProfilePage.tsx`
- `web/src/components/statGroups.ts`

Display:

- Level badge near display name.
- Total XP.
- Progress bar to next level.
- Add XP/level to headline or overview stats.

Update leaderboard UI:

- `web/src/pages/LeaderboardPage.tsx`
- Add sort option `XP`.
- Add XP or Level column.

## Mobile plan

Mirror web API and UI updates.

Update:

- `mobile/src/api/stats.ts`
- `mobile/app/(app)/profile/[id].tsx`
- relevant mobile stat components

Display:

- Level badge.
- Total XP.
- Progress to next level.

## Test plan

### Backend unit tests

Add tests for XP helper:

- rank 1 human-only win
- rank 2/3/4 bonuses
- zero penalty bonus
- low penalty bonus
- bot-mixed multiplier
- minimum XP floor
- level calculation boundaries
- XP progress calculation

### Backend integration tests

Add or extend `SaveGame` tests:

- registered player receives XP
- guest does not receive XP
- bot does not receive XP
- `player_xp_events` row is inserted
- `user_stats.xp` is updated
- leaderboard sort by XP works

### Frontend checks

Run:

```bash
cd web && npm run lint
cd web && npm run build
```

### Mobile checks

Run:

```bash
cd mobile && npx tsc --noEmit
cd mobile && npm test
```

### Full project checks

Run if practical:

```bash
make test
make lint
```

## Implementation order

1. Add database migration.
2. Add XP helper and unit tests.
3. Integrate XP award into `SaveGame` transaction.
4. Expose XP and level in stats APIs.
5. Add leaderboard sort by XP.
6. Update web stats/profile/leaderboard UI.
7. Update mobile stats/profile UI.
8. Run backend, web, and mobile verification commands.

## Open follow-ups

- Decide whether XP history endpoint is needed in v1.
- Tune XP formula after observing real match data.
- Consider backfilling XP for existing games from `game_players` (one-off migration) so early players are not at 0; otherwise XP starts accruing only from the first game after release.
