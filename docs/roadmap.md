# Roadmap

The MVP was built as vertical slices, each a self-contained GitHub issue in
dependency order. All MVP slices are now implemented; subsequent hardening work
is listed below. Future ideas live under [Backlog](#backlog).

## Status Legend

| Symbol | Meaning |
|---|---|
| ✅ | Implemented |
| 🔲 | Not started |

---

## Implemented

### Foundation

| # | Title |
|---|---|
| [#2](https://github.com/faytranevozter/7spade/issues/2) | Monorepo + Docker Compose scaffold |
| [#3](https://github.com/faytranevozter/7spade/issues/3) | Game Engine: core rules |
| [#4](https://github.com/faytranevozter/7spade/issues/4) | Game Engine: Ace closing rule |

### Authentication

| # | Title |
|---|---|
| [#5](https://github.com/faytranevozter/7spade/issues/5) | Guest auth + JWT |
| [#6](https://github.com/faytranevozter/7spade/issues/6) | Email/password auth (with refresh-token rotation) |
| [#7](https://github.com/faytranevozter/7spade/issues/7) | OAuth: Google + GitHub |
| [#8](https://github.com/faytranevozter/7spade/issues/8) | OAuth: Telegram |

### Lobby

| # | Title |
|---|---|
| [#9](https://github.com/faytranevozter/7spade/issues/9) | Room creation + lobby (ready-up, host start, bot backfill) |

### Real-Time Gameplay

| # | Title |
|---|---|
| [#10](https://github.com/faytranevozter/7spade/issues/10) | Game State Store (Redis) |
| [#11](https://github.com/faytranevozter/7spade/issues/11) | WebSocket game server: gameplay loop |
| [#12](https://github.com/faytranevozter/7spade/issues/12) | React game board: live gameplay |
| [#13](https://github.com/faytranevozter/7spade/issues/13) | Turn timer + auto-play bot |
| [#14](https://github.com/faytranevozter/7spade/issues/14) | Disconnect + reconnect |

### End of Game

| # | Title |
|---|---|
| [#15](https://github.com/faytranevozter/7spade/issues/15) | Game over + scoring |
| [#16](https://github.com/faytranevozter/7spade/issues/16) | Game history |
| [#17](https://github.com/faytranevozter/7spade/issues/17) | Rematch |

> Live room state is persisted to **Redis as room snapshots** (`services/ws/store`),
> so rooms survive a WS restart and are rehydrated lazily on the next reconnect.
> See [Architecture](./architecture.md#state-storage).

---

## Post-MVP Hardening

Work done after the MVP to fix bugs and tighten behaviour:

- **Ace-close UX** — closing a suit with an Ace is a first-class action: the
  client highlights a closable Ace, prompts low vs. high in a modal when both
  ends are legal, and the final board renders the closing Ace. Aces are
  excluded from normal sequence plays so they can never corrupt a suit's range.
- **Realtime waiting room** — explicit "Leave room" removes a player instantly
  (no reconnect grace); disconnected lobby players are excluded from
  `can_start`; the host can no longer start a game seating a phantom player.
- **Lobby reconnect grace** — an accidental drop (refresh/network blip) holds
  the seat for ~10s so the player can reconnect to the same slot.
- **Orphan-room reconcile** — the WS service periodically reports its live room
  set to the API, which deletes presence-less `waiting` rooms so abandoned
  lobbies don't linger in the public list.
- **Durable room snapshots** — the WS service persists each room to Redis as a
  snapshot after every change (async, off the room lock) and rehydrates rooms
  on reconnect, so games survive a WS restart. Redis is now required by the WS
  service.
- **Internal-API guard** — the API's `/internal/*` service-to-service endpoints
  accept an optional shared-secret header (`INTERNAL_API_SECRET`).
- **Finished-room results** — reconnecting to a finished room (or hitting its
  URL directly) shows the results screen with the final board instead of an
  empty/phantom game.
- **Auth context + route guards** — a shared `AuthProvider` keeps auth state in
  sync across the SPA; authenticated users are redirected away from the
  login/register pages, and a "Sign out" control clears the session.
- **Bot difficulty levels** — room creation supports `easy`, `medium`, and
  `hard` bot behaviour, persisted on the room and Redis snapshot. `medium`
  is the default for bot backfill; web and mobile expose the
  selector and show the selected difficulty in room surfaces.

### Mobile App

A React Native + Expo client (`mobile/`) for iOS and Android, with full feature
parity to the web SPA over the same API + WebSocket contracts:

- Reuses the web app's pure logic (types, card/board math, catalogs, JWT claims,
  API client, the `useGameSocket` reducer + board builder); the UI is rebuilt
  with React Native primitives + NativeWind, navigated via Expo Router.
- Auth tokens persist in `expo-secure-store` and refresh transparently on launch;
  OAuth uses an `expo-auth-session` deep link (`sevenspade://`).
- Socket hooks add backoff auto-reconnect and reconnect-on-foreground.
- Required two small, additive backend changes (no WS changes): token-in-body
  `/refresh` + `/auth/logout` for cookieless native clients, and an allowlisted
  `redirect_uri` passthrough on the OAuth endpoints.
- See [Mobile App](./mobile.md) for the full architecture.

### Self-Profile + Editable Display Name

A "My profile" screen on both clients (`/me` on web, `/(app)/me` on mobile):
own avatar, display name, lifetime stats, and achievements, reachable from a new
header entry. Guests see a limited view with a register prompt. Registered users
can edit their display name via a new `PATCH /me` endpoint, which updates
`users.display_name` and re-issues the access JWT (the name lives in the token,
read by the WS server to label the seat). Public profiles for other players stay
at `/players/:id` and `/(app)/profile/[id]`.

---

## Backlog

All backlog items are tracked as GitHub issues in the [Post-MVP Features](https://github.com/faytranevozter/7spade/milestone/1) milestone.

### Priority Features

| 🔲 | Issue | Effort |
|---|---|---|
| ✅ | [#38](https://github.com/faytranevozter/7spade/issues/38) Practice Mode (Solo vs Bots) | Low |
| 🔲 | [#39](https://github.com/faytranevozter/7spade/issues/39) Quick Play / Auto-Matchmaking | Medium |
| 🔲 | [#40](https://github.com/faytranevozter/7spade/issues/40) In-Game Tutorial / Onboarding | Low-Medium |
| ✅ | [#41](https://github.com/faytranevozter/7spade/issues/41) Bot Difficulty Levels | Low |
| ✅ | [#42](https://github.com/faytranevozter/7spade/issues/42) Password Reset & Email Verification | Low |

### Engagement Features

| 🔲 | Issue | Effort |
|---|---|---|
| ✅ | [#43](https://github.com/faytranevozter/7spade/issues/43) [Seasons & ELO Rating](./specs/seasons-and-elo.md) | High |
| 🔲 | [#44](https://github.com/faytranevozter/7spade/issues/44) Game Replay / Move History | Medium-High |
| 🔲 | [#45](https://github.com/faytranevozter/7spade/issues/45) Party / Group Queue | Medium |
| ✅ | [#46](https://github.com/faytranevozter/7spade/issues/46) Profile Stat Comparison (You vs Player) | Low |
| 🔲 | [#47](https://github.com/faytranevozter/7spade/issues/47) Spectator Chat & Emotes | Low |

### Quality of Life

| 🔲 | Issue | Effort |
|---|---|---|
| 🔲 | [#48](https://github.com/faytranevozter/7spade/issues/48) Card Play Animations | Medium |
| 🔲 | [#50](https://github.com/faytranevozter/7spade/issues/50) Push Notifications (Mobile) | Medium |
| ✅ | [#49](https://github.com/faytranevozter/7spade/issues/49) Configurable Leaderboard Sort | Low |
| ✅ | [#51](https://github.com/faytranevozter/7spade/issues/51) User Search Beyond Exact Match | Low |
| 🔲 | [#52](https://github.com/faytranevozter/7spade/issues/52) Custom Avatar Upload | Low |

### Operational

| 🔲 | Issue | Effort |
|---|---|---|
| 🔲 | [#53](https://github.com/faytranevozter/7spade/issues/53) Rate Limiting | Medium |
| 🔲 | [#54](https://github.com/faytranevozter/7spade/issues/54) Admin Panel & Moderation | High |
| 🔲 | [#55](https://github.com/faytranevozter/7spade/issues/55) Account Deletion | Low-Medium |

---

## Historical Build Order

The MVP slices were implemented in this dependency order:

```
#2 (scaffold)
├── #3 (game engine core)
│   ├── #4 (ace closing rule)
│   │   └── #11 (WS gameplay loop) ◄─ also needs #9, #10
│   └── #10 (Redis state store)
│       └── #11
├── #5 (guest auth)
│   ├── #6 (email auth)
│   │   ├── #7 (Google/GitHub OAuth)
│   │   │   └── #8 (Telegram OAuth)
│   │   └── #16 (game history) ◄─ also needs #15
│   └── #9 (rooms + lobby)
│       └── #11
└── #11
    ├── #12 (React game board)
    │   └── #15 (game over + scoring)
    │       ├── #16
    │       └── #17 (rematch)
    └── #13 (turn timer + bot)
        └── #14 (disconnect/reconnect)
```
