# Roadmap

Implementation is broken into vertical slices, each a self-contained GitHub issue. Issues are listed in dependency order — each slice builds on the ones it is blocked by.

## Status Legend

| Symbol | Meaning |
|---|---|
| 🔲 | Not started |
| ✅ | Complete |

---

## Slices

### Foundation

| # | Title | Status | Blocked by |
|---|---|---|---|
| [#2](https://github.com/faytranevozter/7spade/issues/2) | Monorepo + Docker Compose scaffold | 🔲 | — |
| [#3](https://github.com/faytranevozter/7spade/issues/3) | Game Engine: core rules | 🔲 | #2 |
| [#4](https://github.com/faytranevozter/7spade/issues/4) | Game Engine: Ace closing rule | 🔲 | #3 |

### Authentication

| # | Title | Status | Blocked by |
|---|---|---|---|
| [#5](https://github.com/faytranevozter/7spade/issues/5) | Guest auth + JWT | 🔲 | #2 |
| [#6](https://github.com/faytranevozter/7spade/issues/6) | Email/password auth | 🔲 | #5 |
| [#7](https://github.com/faytranevozter/7spade/issues/7) | OAuth: Google + GitHub | 🔲 | #6 |
| [#8](https://github.com/faytranevozter/7spade/issues/8) | OAuth: Telegram | 🔲 | #7 |

### Lobby

| # | Title | Status | Blocked by |
|---|---|---|---|
| [#9](https://github.com/faytranevozter/7spade/issues/9) | Room creation + lobby | 🔲 | #5 |

### Real-Time Gameplay

| # | Title | Status | Blocked by |
|---|---|---|---|
| [#10](https://github.com/faytranevozter/7spade/issues/10) | Game State Store (Redis) | 🔲 | #3 |
| [#11](https://github.com/faytranevozter/7spade/issues/11) | WebSocket game server: basic gameplay loop | 🔲 | #3, #4, #9, #10 |
| [#12](https://github.com/faytranevozter/7spade/issues/12) | React game board: live gameplay | 🔲 | #11 |
| [#13](https://github.com/faytranevozter/7spade/issues/13) | Turn timer + auto-play | 🔲 | #11 |
| [#14](https://github.com/faytranevozter/7spade/issues/14) | Disconnect + reconnect | 🔲 | #13 |

### End of Game

| # | Title | Status | Blocked by |
|---|---|---|---|
| [#15](https://github.com/faytranevozter/7spade/issues/15) | Game over + scoring | 🔲 | #12 |
| [#16](https://github.com/faytranevozter/7spade/issues/16) | Game history | 🔲 | #6, #15 |
| [#17](https://github.com/faytranevozter/7spade/issues/17) | Rematch | 🔲 | #15 |

---

## Dependency Graph

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
