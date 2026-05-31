# Spec: Player Stats & Leaderboard

Status: Implemented
Owner: —
Related: [Architecture](../architecture.md) · [HTTP API](../api.md) · [Roadmap](../roadmap.md)

## 1. Overview

Add per-player lifetime statistics and a public leaderboard derived from
completed games. Every finished game already records a row per player in
`game_players` (rank, penalty points, winner flag); this feature aggregates that
data into durable per-user totals and exposes them through new read APIs and
frontend views.

### Goals

- Show a registered player their own lifetime stats (games played, wins, win
  rate, average penalty, best round).
- A public, paginated leaderboard ranking players by win rate.
- Public per-player profile pages reachable from the leaderboard.

### Non-goals

- Skill rating / ELO, matchmaking by rating.
- Seasons or time-windowed leaderboards (all-time only for v1).
- Stats for guest sessions (guests have no durable identity).
- Real-time push of stat changes (stats refresh on navigation / refetch).

## 2. Metrics

All metrics are derived from a player's `game_players` rows across completed
games. Only rows with a non-null `user_id` (registered users) count — guest
rows are ignored.

| Metric | Definition |
|---|---|
| `games_played` | Count of finished games the user participated in |
| `wins` | Count of those games where `is_winner = true` |
| `win_rate` | `wins / games_played` (0 when `games_played = 0`) |
| `avg_penalty` | Mean `penalty_points` across the user's games |
| `best_penalty` | Lowest single-game `penalty_points` the user has scored |

**Tie handling.** Seven Spade awards shared wins: when several players tie for
the lowest penalty they all have `is_winner = true` and `rank = 1`. Each such
player's `wins` increments — shared wins count as wins for everyone tied.

`win_rate` and `avg_penalty` are **derived at read time** from stored integer
counters (`wins`, `games_played`, `total_penalty`) rather than stored as
floats, to avoid rounding drift.

## 3. Data Model

A denormalized `user_stats` table holds per-user counters, updated
transactionally whenever a game is saved. This keeps leaderboard reads cheap at
any table size (no `GROUP BY` over the full `game_players` history per request).

### Migration `005_user_stats.sql`

```sql
CREATE TABLE IF NOT EXISTS user_stats (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    games_played  INTEGER NOT NULL DEFAULT 0,
    wins          INTEGER NOT NULL DEFAULT 0,
    total_penalty BIGINT  NOT NULL DEFAULT 0,   -- sum of penalty_points; avg derived
    best_penalty  INTEGER NULL,                 -- lowest single-game penalty, null until first game
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Supports the min-games qualification filter and ordering scans.
CREATE INDEX IF NOT EXISTS idx_user_stats_games_played ON user_stats(games_played);

-- One-time backfill from existing recorded games (registered players only).
INSERT INTO user_stats (user_id, games_played, wins, total_penalty, best_penalty, updated_at)
SELECT
    gp.user_id,
    COUNT(*)                              AS games_played,
    COUNT(*) FILTER (WHERE gp.is_winner)  AS wins,
    COALESCE(SUM(gp.penalty_points), 0)   AS total_penalty,
    MIN(gp.penalty_points)                AS best_penalty,
    NOW()
FROM game_players gp
WHERE gp.user_id IS NOT NULL
GROUP BY gp.user_id
ON CONFLICT (user_id) DO NOTHING;
```

> Migrations are embedded and auto-applied on API startup (see
> [development.md](../development.md)). The backfill makes the table consistent
> with games recorded before this feature shipped.

### Update path (transactional with the game save)

`SaveGame` already inserts the `games` row and one `game_players` row per player
inside a transaction. Within that same transaction, for each player that has a
non-null `user_id`, upsert their stats:

```sql
INSERT INTO user_stats (user_id, games_played, wins, total_penalty, best_penalty, updated_at)
VALUES ($1, 1, $2, $3, $3, NOW())
ON CONFLICT (user_id) DO UPDATE SET
    games_played  = user_stats.games_played + 1,
    wins          = user_stats.wins + $2,
    total_penalty = user_stats.total_penalty + $3,
    best_penalty  = LEAST(COALESCE(user_stats.best_penalty, $3), $3),
    updated_at    = NOW();
```

`$2` is `1` when `is_winner` else `0`; `$3` is the game's `penalty_points`.
Because it runs in the existing save transaction, stats never diverge from the
underlying `game_players` rows on the happy path.

### Drift recovery

If the two ever diverge (e.g. a manual data fix), `user_stats` can be rebuilt
from `game_players` with a recompute query — the same aggregate as the backfill,
wrapped in `TRUNCATE user_stats; INSERT … SELECT … GROUP BY`. Documented as an
operational runbook step, not an automatic process.

## 4. API

Three new read endpoints on the HTTP API. The leaderboard and public profile
are unauthenticated (read-only public data); the personal stats endpoint
requires a registered (non-guest) session, mirroring `GET /history`.

### `GET /leaderboard`

Public. Ranked, paginated list of qualifying players.

**Query params**

| Param | Default | Notes |
|---|---|---|
| `page` | 1 | 1-based |
| `per_page` | 10 | capped at 50 (mirrors history) |

**Ranking.** Players with `games_played >= LEADERBOARD_MIN_GAMES` (default 5)
qualify. Ordered by `win_rate` descending, tie-broken by `games_played`
descending, then `avg_penalty` ascending (lower is better), then `user_id` for
stability. Sub-threshold players are omitted from the ranked list. `rank` is the
1-based position across the full qualifying set (not just the page).

**Response**
```json
{
  "entries": [
    {
      "rank": 1,
      "user_id": "…",
      "display_name": "Alice",
      "games_played": 42,
      "wins": 30,
      "win_rate": 0.714,
      "avg_penalty": 12.4,
      "best_penalty": 3
    }
  ],
  "total": 87,
  "page": 1,
  "min_games": 5
}
```

`total` is the count of qualifying players; `min_games` echoes the active
threshold so the client can explain why a player may be absent.

### `GET /stats`

Authenticated (registered users only; guests receive `401` with
`"Logged-in user required"`, exactly as `GET /history` does). Returns the
caller's own stats, plus their current leaderboard rank when they qualify.

**Response**
```json
{
  "user_id": "…",
  "display_name": "Alice",
  "games_played": 42,
  "wins": 30,
  "win_rate": 0.714,
  "avg_penalty": 12.4,
  "best_penalty": 3,
  "rank": 1,
  "qualified": true
}
```

A user with no games yet returns zeroed counters, `best_penalty: null`,
`rank: null`, `qualified: false`.

### `GET /users/:id/stats`

Public. Same body shape as `GET /stats` for the given user id. Returns `404` if
the user does not exist or has no `user_stats` row (never played a recorded
game). `display_name` is read from the `users` table so it reflects the player's
current name.

## 5. Server Design

Follows the existing layering (`repository` → `handler` → `server` router),
mirroring `GetPlayerHistory`/`HistoryHandler`.

### Repository (`internal/repository`)

- `UpsertUserStats(tx *sql.Tx, userID uuid.UUID, isWinner bool, penalty int) error`
  — the upsert above; called from inside `SaveGame`'s transaction once per
  registered player. Guests (empty `UserID`) are skipped.
- `GetLeaderboard(db, page, perPage, minGames int) (entries []LeaderboardEntry, total int, err error)`
  — qualifying-count query + a windowed page query computing `win_rate`,
  `avg_penalty`, and `rank` (e.g. via `ROW_NUMBER()` ordered by the ranking
  rule). `display_name` joined from `users`.
- `GetUserStats(db, userID) (*UserStats, bool, error)` — single-row fetch plus a
  derived rank (count of qualifying users ranked above, when the user
  qualifies).

### Handler (`internal/handler/stats.go`)

`StatsHandler{ DB }` with `Leaderboard`, `Me`, and `User` methods. `Me`
extracts claims and rejects guests like `HistoryHandler.List`. Reuses
`positiveQueryInt` and the 50-cap for pagination. The min-games threshold is
read once from config.

### Router (`internal/server/router.go`)

```go
r.GET("/leaderboard", statsHandler.Leaderboard)   // public
r.GET("/users/:id/stats", statsHandler.User)      // public
authed.GET("/stats", statsHandler.Me)             // registered users only
```

`SaveGame` keeps its current signature; the stats upsert is internal to its
transaction, so the WS `/internal/games` flow is unchanged.

### Config

`LEADERBOARD_MIN_GAMES` (default `5`) added to `internal/config`. No new env is
required to run; the default applies when unset.

## 6. Frontend

Mirrors the existing `api/history.ts` + `HistoryPage` patterns and the auth /
route-guard conventions in `App.tsx`.

### API client (`web/src/api/stats.ts`)

- `getLeaderboard(token, page, perPage)` → leaderboard response.
- `getMyStats(token)` → current user's stats (authenticated).
- `getUserStats(token, userId)` → public profile stats.

(Token is optional for the public calls; passed when available.)

### Pages & nav

- **Leaderboard page** (`/leaderboard`): a ranked table (rank, player,
  games, win rate, avg penalty, best). Pagination controls like HistoryPage.
  Each row links to that player's profile. Add a "Leaderboard" entry to the
  authenticated header nav (next to "Lobby" / "My Games").
- **Profile page** (`/players/:id`): renders `getUserStats` — the same stat
  cards as the personal dashboard, read-only. Public route (no auth guard).
- **My stats**: surface `getMyStats` either as a panel on the existing History
  ("My Games") page or a small stats header on it, so a logged-in player sees
  their win rate / rank alongside recent games.

### States

Loading, empty ("No ranked players yet — play a few games to appear"),
sub-threshold ("Play N games to join the leaderboard"), and error states, all
consistent with current pages.

## 7. Edge Cases

- **Guests**: excluded from aggregation (`user_id` is null on their rows) and
  from all stats endpoints (`/stats` returns 401 for guest tokens).
- **User deletion**: `game_players.user_id` is `ON DELETE SET NULL`, so a
  deleted user's historical rows stop aggregating; `user_stats.user_id` is
  `ON DELETE CASCADE`, so their stats row is removed. The leaderboard simply
  drops them.
- **Display-name collisions / renames**: stats are keyed by `user_id`, and
  `display_name` is read live from `users`, so renames and duplicate names are
  handled correctly.
- **Zero games / sub-threshold**: zeroed stats with `best_penalty: null`,
  `rank: null`, `qualified: false`; omitted from the ranked leaderboard.
- **Shared wins**: every tied winner increments `wins` — win rate can exceed
  intuition in heavily-tied groups; documented, not "corrected".
- **Pagination**: `per_page` capped at 50; `rank` is global across qualifiers,
  not per-page.
- **Scale**: reads hit the small `user_stats` table (one row per user), not the
  unbounded `game_players` history.

## 8. Testing

- **Go repository**: `UpsertUserStats` correctness inside a transaction —
  increments, shared-win counting, `best_penalty` MIN behavior, guest skip;
  `GetLeaderboard` ordering + threshold + tie-breaks + pagination;
  `GetUserStats` for existing / missing / sub-threshold users; backfill query
  equals incremental result for the same games.
- **Go handler**: `/stats` rejects guests (401), accepts registered users;
  `/leaderboard` and `/users/:id/stats` are reachable unauthenticated; bad
  user id → 400/404; `per_page` cap enforced.
- **Web**: `api/stats.ts` request shapes; Leaderboard page renders rows,
  paginates, links to profiles, and shows empty/sub-threshold states; profile
  page renders stats and 404s gracefully.
- Run `make -C services/api test`, `make -C services/ws test` (unchanged), and
  `cd web && npm test && npm run lint && npm run build`.

## 9. Rollout

1. Ship migration `005_user_stats.sql` (table + index + backfill). Auto-applied
   on API startup.
2. Deploy API with the new endpoints and the `SaveGame` upsert.
3. Deploy the frontend leaderboard / profile / stats UI.
4. No breaking changes: existing endpoints and the WS `/internal/games` flow are
   untouched; `LEADERBOARD_MIN_GAMES` is optional.

## 10. Open Questions / Future Work

- **Seasons / time windows** — period-scoped leaderboards (weekly/monthly).
- **Skill rating (ELO)** — rating-based ranking and matchmaking.
- **Caching** — a short-TTL cache or materialized view if leaderboard reads
  ever get hot (the denormalized table should defer this need).
- **Head-to-head / richer stats** — streaks, opponent records, rank
  distribution charts.
- **Configurable ranking metric** — expose sort by avg penalty or total wins.




