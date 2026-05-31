# Spec: Seasons & Skill Rating (ELO)

Status: Proposed
Owner: —
Related: [Player Stats & Leaderboard](./stats-and-leaderboard.md) · [HTTP API](../api.md) · [Architecture](../architecture.md)

## 1. Overview

Two related extensions to the stats system, both listed as future work in the
[Stats & Leaderboard spec](./stats-and-leaderboard.md#10-open-questions--future-work):

- **Seasons** — time-windowed leaderboards (e.g. monthly), so rankings reset and
  reward recent play instead of all-time accumulation.
- **Skill rating (ELO)** — a per-player rating updated after each game, enabling
  a skill-ordered leaderboard and (optionally) rating-based matchmaking.

They share infrastructure (per-game aggregation, the leaderboard read path) and
are phased so seasons can ship before rating.

### Goals

- Period-scoped leaderboards alongside the existing all-time one.
- A skill rating that reflects finishing position, updated transactionally with
  the game save.
- Keep reads cheap (denormalized, mirroring `user_stats`).

### Non-goals

- Rewards/cosmetics for season placement (future).
- Anti-smurf / anti-boost detection.
- Replacing the all-time leaderboard (seasons are additive).
- Live matchmaking queues (v1 matchmaking, if built, is lobby-assist only).

## 2. Phase A — Seasons

### Concept

A **season** is a named time window. Stats are bucketed per season so a
leaderboard can be scoped to one. The all-time leaderboard remains unchanged.

### Data model (migration `006_seasons.sql`)

```sql
CREATE TABLE IF NOT EXISTS seasons (
    id         TEXT PRIMARY KEY,          -- e.g. '2026-05'
    label      TEXT NOT NULL,             -- 'May 2026'
    started_at TIMESTAMP NOT NULL,
    ended_at   TIMESTAMP                  -- null = current/open season
);

CREATE TABLE IF NOT EXISTS season_user_stats (
    season_id     TEXT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    games_played  INTEGER NOT NULL DEFAULT 0,
    wins          INTEGER NOT NULL DEFAULT 0,
    total_penalty BIGINT  NOT NULL DEFAULT 0,
    best_penalty  INTEGER NULL,
    PRIMARY KEY (season_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_season_user_stats_lookup
    ON season_user_stats(season_id, games_played);
```

`season_user_stats` mirrors `user_stats` but keyed by `(season_id, user_id)`.
The current season is resolved at save time (the open season, or by date
bucket). `SaveGame`'s existing per-player upsert gains a parallel upsert into
`season_user_stats` for the active season — same transaction, same shape.

### API & frontend

- `GET /leaderboard?season=<id>` — scopes the leaderboard query to
  `season_user_stats` for that season; omitting `season` keeps all-time
  behavior. A `GET /seasons` lists available seasons (id, label, window).
- `GetLeaderboard` gains a season variant (or a `seasonID` arg, empty = all-time)
  reusing the same ranking rule and `LEADERBOARD_MIN_GAMES` threshold.
- Frontend: a season selector on the leaderboard page; the default is the
  current season (or all-time — see Open Questions). Per-season stats can also
  appear on the profile.

### Season lifecycle

Seasons are created/closed by an operational process (a scheduled job or manual
runbook step inserting the next `seasons` row and setting `ended_at` on the
prior one). No automatic rollover logic is required in the game path; the active
season is simply "the row with `ended_at IS NULL`". Date-bucketed ids (e.g.
`2026-05`) make this deterministic.

## 3. Phase B — Skill Rating (ELO)

### Concept

Each registered player has a numeric rating (default e.g. 1000). After each
finished game the four players' ratings are adjusted based on finishing position
(rank), so beating stronger players gains more.

### Multiplayer rating method

Standard 1v1 ELO doesn't directly apply to a 4-player free-for-all. Adapt via
**pairwise expansion**: treat the final ranking as C(4,2)=6 pairwise outcomes
(each player vs. each other), where the better-ranked player "wins" the pair
(ties split). Sum the per-pair ELO deltas for each player and apply once.
`expected = 1 / (1 + 10^((opp - me)/400))`; `delta = K * (score - expected)`
summed over opponents, with a modest `K` (e.g. 24). Shared ranks (tied winners)
score 0.5 against each other. The exact constants/method are an Open Question;
pairwise is chosen for being simple, well-understood, and order-independent.

### Data model (migration `007_ratings.sql`)

```sql
ALTER TABLE user_stats ADD COLUMN IF NOT EXISTS rating INTEGER NOT NULL DEFAULT 1000;
```

Rating lives on `user_stats` (one row per user, already joined by the
leaderboard). It is updated inside `SaveGame`'s transaction: read the four
players' current ratings, compute deltas from the ranks, write back. Guests/bots
(no `user_id`) are excluded from the calculation — only registered players'
ratings move, and a game's pairwise set is restricted to registered
participants (Open Question: how bots affect rating).

### API & frontend

- The leaderboard gains a rating-ordered mode (e.g. `?metric=rating`), tying
  into the stats spec's "configurable ranking metric" future-work item.
- `/stats` and `/users/:id/stats` include `rating`.
- Frontend: show rating on the profile and as a leaderboard column/sort option.

### Optional — rating-based matchmaking

A "ranked" lobby option that groups players of similar rating. v1 scope is
lobby-assist only (suggest/auto-join a room with nearby ratings); a true
matchmaking queue is future work.

## 4. Edge Cases

- **Guests / bots**: excluded from season stats and rating (no `user_id`),
  consistent with all-time stats.
- **Season boundaries**: a game counts toward whichever season is active at save
  time; no retroactive reassignment.
- **Rating cold start**: new players start at the default; high variance early —
  a provisional period (larger K for first N games) is an Option.
- **Drift recovery**: `season_user_stats` can be recomputed from `game_players`
  filtered by the season window, like the all-time backfill. Ratings are
  order-dependent and can only be rebuilt by replaying games chronologically.
- **Backfill**: existing recorded games predate seasons/ratings; a one-time
  backfill (assign to date-bucketed seasons; replay for ratings) is optional.

## 5. Testing

- Seasons: per-season upsert in `SaveGame`; `GetLeaderboard(season)` scoping,
  threshold, and ranking match the all-time rules; `GET /seasons` lists windows.
- Rating: pairwise delta math (symmetry — winners gain what losers lose within a
  pair; ties net zero); rank-1-of-4 gains, rank-4 loses; registered-only set.
- Handler: `?season=` and `?metric=rating` accepted; bad season → empty/404 per
  contract.
- Web: season selector, rating column/sort, profile rating.
- Run `make -C services/api test`, `make -C services/ws test` (unchanged), and
  `cd web && npm test && npm run lint && npm run build`.

## 6. Rollout

1. **Phase A**: migration `006` (seasons), per-season upsert in `SaveGame`,
   `?season=` leaderboard + `/seasons`, frontend selector.
2. **Phase B**: migration `007` (rating column), rating update in `SaveGame`,
   rating leaderboard mode + profile rating.
3. **Phase C (optional)**: rating-based matchmaking in the lobby.
4. All additive; the all-time leaderboard and existing endpoints are untouched.

## 7. Open Questions / Future Work

- **Default leaderboard scope** — current season vs. all-time as the landing
  view.
- **Multiplayer rating formula** — pairwise ELO vs. alternatives (Glicko-2,
  TrueSkill); exact `K`, provisional period, and how bot seats factor in.
- **Season cadence & rewards** — monthly vs. custom; placement rewards/cosmetics.
- **Matchmaking** — full queue vs. lobby-assist; rating bands; wait-time vs.
  match-quality tradeoffs.
- **Decay** — rating/season-rank decay for inactivity.
