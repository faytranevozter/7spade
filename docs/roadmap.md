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

> Note: live game state is held **in-memory** in the WS process rather than in
> Redis. The original "Game State Store (Redis)" slice (#10) shipped as the
> `services/ws/store` package but is not currently wired into the running
> server — see [Architecture](./architecture.md#state-storage).

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
- **Internal-API guard** — the API's `/internal/*` service-to-service endpoints
  accept an optional shared-secret header (`INTERNAL_API_SECRET`).
- **Finished-room results** — reconnecting to a finished room (or hitting its
  URL directly) shows the results screen with the final board instead of an
  empty/phantom game.
- **Auth context + route guards** — a shared `AuthProvider` keeps auth state in
  sync across the SPA; authenticated users are redirected away from the
  login/register pages, and a "Sign out" control clears the session.

---

## Backlog

No additional slices are currently tracked. New ideas should be filed as GitHub
issues and linked here.

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
