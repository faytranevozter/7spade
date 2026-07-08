# Spec: Achievements & Badges

Status: Implemented
Owner: —
Related: [Player Stats & Leaderboard](./stats-and-leaderboard.md) · [Architecture](../architecture.md) · [HTTP API](../api.md)

## 1. Overview

Award players durable **achievements** (badges) derived from their recorded
games. Most achievements fall out of data the app already stores — the
`game_players` rows and the denormalized `user_stats` counters — so evaluation
piggybacks on the existing transactional game-save path. Earned badges surface
on the public profile and as a toast when freshly earned.

### Goals

- Reward milestones and notable performances to deepen engagement.
- Evaluate cheaply and transactionally alongside the existing stats upsert.
- Show a player their badges (and others') on profile pages.

### Non-goals

- Real-time achievement feeds or social sharing.
- Achievements for guests (no durable identity) or bots.
- Secret/hidden achievements UI (all are listed; v1 keeps it simple).
- Points/XP economy — badges are boolean earned/not-earned for v1.

## 2. Catalog (v1 + v2)

| ID | Name | Condition |
|---|---|---|
| `first_win` | First Blood | First game with `is_winner = true` |
| `games_10` / `games_50` / `games_100` | Regular / Veteran / Centurion | `games_played` reaches 10 / 50 / 100 |
| `streak_3` / `streak_5` | On a Roll / Unstoppable | 3 / 5 wins in a row |
| `perfect_round` | Flawless | A game finished with `penalty_points = 0` |
| `shared_win` | Good Company | Won a game tied with others (shared win) |
| `wins_50` / `wins_100` | Champion / Legend | `wins` reaches 50 / 100 |
| `streak_10` / `streak_15` | Legendary / Mythical | 10 / 15 wins in a row |
| `firsts_50` / `firsts_100` | Sovereign / Emperor | `first_place_count` reaches 50 / 100 |
| `zero_penalty_games_10` | Zen Master | `zero_penalty_games` reaches 10 |
| `games_200` | Lifetimer | `games_played` reaches 200 |
| `human_only_25` | Social Butterfly | `human_only_games` reaches 25 |
| ~~`top_10`~~ | ~~Contender~~ | Deferred — see §10 (rank-derived, not shipped in v1) |

IDs are stable; names/descriptions are presentation-only and live in a shared
catalog (server + client kept in sync, mirroring the emote allowlist pattern).

## 3. Data Model

### Migration `006_user_achievements.sql`

```sql
CREATE TABLE IF NOT EXISTS user_achievements (
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id TEXT NOT NULL,
    earned_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_id)
);

CREATE INDEX IF NOT EXISTS idx_user_achievements_user_id ON user_achievements(user_id);
```

The composite PK makes awarding idempotent (`ON CONFLICT DO NOTHING`), so
re-evaluating a previously-earned badge is a no-op.

### Streak support

Win streaks need state that a single game row doesn't carry. Add a
`current_streak INTEGER NOT NULL DEFAULT 0` column to `user_stats` (migration
`006`), incremented on a win and reset to 0 on a loss inside the same
`UpsertUserStats` transaction. `streak_3` / `streak_5` are awarded when
`current_streak` crosses the threshold. (Alternative: a bounded lookback over
the last N `game_players` rows; the counter is cheaper and matches the existing
denormalized-counter philosophy.)

## 4. Evaluation Path

Achievements are evaluated **inside `SaveGame`'s transaction**, after the
existing `UpsertUserStats` call, for each registered player. The evaluator has
everything it needs in-transaction: the player's just-saved `game_players` row
(penalty, is_winner) and the updated `user_stats` counters (games_played, wins,
current_streak). For each newly-satisfied condition it issues:

```sql
INSERT INTO user_achievements (user_id, achievement_id)
VALUES ($1, $2) ON CONFLICT DO NOTHING;
```

`top_10` is rank-derived; it can be evaluated here using the same ranking rule
as the leaderboard, or deferred to a lighter periodic job if per-save ranking
is undesirable (see Open Questions).

The set of achievements newly earned in this save is returned so the WS layer
(which calls `/internal/games`) can optionally relay them to the client for a
toast — or the client simply diffs its badge list on next fetch (v1 default,
no protocol change).

## 5. API

- `GET /users/:id/achievements` — public; returns earned badges with
  `earned_at`, plus (for convenience) the full catalog so the client can render
  locked/unlocked states. Mirrors `GET /users/:id/stats`.
- Optionally fold an `achievements` array into the existing `/stats` and
  `/users/:id/stats` responses so the profile renders in one fetch.

Repository: `GetUserAchievements(db, userID) ([]Achievement, error)`;
`AwardAchievements(tx, userID, ids)` called from `SaveGame`. Handler mirrors
`StatsHandler`.

## 6. Frontend (`web`)

- The achievement catalog is server-authoritative. The web client reads the full
  catalog from `GET /users/:id/achievements` and does not keep a bundled fallback,
  so ids, display copy, ordering, and enablement cannot drift from the API DB.
- `api/achievements.ts` client (`getUserAchievements`).
- Profile page (`/players/:id` and the My-Games stats panel): a badge grid
  showing earned badges highlighted and locked ones dimmed with their unlock
  condition.
- Optional "Achievement unlocked" toast after a game, using the existing
  `ToastStack`, if the WS relays newly-earned ids.

## 7. Edge Cases

- **Guests / bots**: skipped (no `user_id`), exactly like stats.
- **Idempotency**: `ON CONFLICT DO NOTHING` means re-awards are harmless.
- **Streak resets**: a loss zeroes `current_streak`; a shared win counts as a
  win for streak purposes (consistent with the stats spec's shared-win rule).
- **Backfill**: existing players won't have historical badges unless backfilled
  (see Open Questions).
- **Catalog unavailable**: if the API returns an empty catalog, the web client
  shows the catalog as unavailable instead of falling back to stale bundled data.

## 8. Testing

- Repository: `AwardAchievements` is idempotent; `first_win`, games-milestone,
  streak increment/reset, and `perfect_round` award correctly given crafted
  game saves; guest skip.
- Handler: `/users/:id/achievements` reachable unauthenticated; bad id → 400;
  unknown user → empty/earned list per chosen contract.
- Web: catalog shape; profile renders earned vs. locked; toast on newly-earned.
- Run `make -C services/api test` and `cd web && npm test && npm run lint && npm run build`.

## 9. Rollout

1. Ship migration `006` (table + `current_streak`).
2. Deploy API with evaluation in `SaveGame` + the new read endpoint.
3. Deploy the frontend badge UI.
4. Optionally run a one-time backfill job.

## 10. Open Questions / Future Work

- **`top_10` not shipped in v1.** The catalog above omits the rank-derived
  `top_10` badge: evaluating it per save means ranking on every game (the cost
  concern below), and it's the only condition not answerable from the player's
  own row + counters. Deferred to a periodic job or a future leaderboard-read
  hook.
- **Folding into `/stats` not done.** v1 ships a dedicated
  `GET /users/:id/achievements` (earned list + catalog) that the profile fetches
  separately, rather than embedding an `achievements` array in the stats
  responses. The extra request keeps the stats payload and its zero-games
  fallback unchanged.
- **Retroactive backfill** — compute historical badges for existing players from
  `game_players` (streaks are order-dependent, so backfill must replay games
  chronologically per user). Not run in v1.
- **`top_10` evaluation cost** — per-save ranking vs. a periodic job.
- **Hidden/secret achievements**, **tiered XP/points**, and **rarity stats**
  (what % of players have each) are future work.
