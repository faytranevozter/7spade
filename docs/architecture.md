# Architecture

Seven Spade is a monorepo with three independently deployable services wired together via Docker Compose.

## System Overview

```
Browser (React + TypeScript)
        │
        ├── HTTP  ──► services/api   (Go)  ──► PostgreSQL 16
        │                                 └──► Redis 7
        └── WS    ──► services/ws    (Go)  ──► Redis 7
```

| Layer | Service | Tech | Port |
|---|---|---|---|
| Frontend | `web/` | React + TypeScript + Vite + Tailwind CSS v4.2 | 3000 |
| HTTP API | `services/api` | Go | 8080 |
| WebSocket game server | `services/ws` | Go | 8081 |
| Relational store | — | PostgreSQL 16 | 5432 |
| Live game state | — | Redis 7 | 6379 |

---

## Services

### HTTP API (`services/api`)

Handles all non-real-time operations:

- User authentication (guest, email/password, Google, GitHub, Telegram OAuth)
- Room creation and lobby management
- Game history persistence and retrieval
- Issues JWTs shared by both services

### WebSocket Game Server (`services/ws`)

Handles everything that requires low-latency, push-based communication:

- Authenticates WebSocket connections via JWT
- Manages per-room hubs that fan-out state snapshots
- Runs the **Game Engine** (pure Go package) to validate and apply moves
- Reads/writes live `GameState` to Redis
- Enforces turn order, turn timer, and auto-play bot

### Frontend (`web/`)

- Single-page React app served via Nginx in production
- Uses Tailwind CSS v4.2 as the primary styling system
- Follows `design/design_system.html` for Seven Spade color, typography, card, board, lobby, and motion tokens
- Communicates with the API over HTTP/JSON and with the WS server over WebSocket
- Stores JWTs in local storage

---

## Data Flow — Gameplay

```
Client                    WS Server                 Redis
  │                          │                        │
  │── play_card ────────────►│                        │
  │                          │── Load(roomID) ───────►│
  │                          │◄── GameState ──────────│
  │                          │                        │
  │                          │ ApplyMove() [engine]   │
  │                          │                        │
  │                          │── Save(roomID, state) ─►│
  │                          │                        │
  │◄── state_update ─────────│ (broadcast to all)     │
```

---

## Game Engine

The Game Engine is a **pure Go package** with no I/O dependencies. It encodes the complete Seven Spade rule set:

| Function | Description |
|---|---|
| `Deal(seed int64)` | Deterministic shuffle and deal; returns which player holds 7♠ |
| `ValidMoves(state, hand)` | Returns legal plays for the current player |
| `ApplyMove(state, playerIndex, card, faceDown bool)` | Validates and applies a move; returns updated state or error |
| `ApplyAceClose(state, suit, method)` | Closes a suit and locks the global closing method |
| `IsGameOver(state)` | Returns true when all hands are empty |
| `CalculateScores(state)` | Sums face-down card values per player |

---

## State Storage

### PostgreSQL

Stores durable data:

| Table | Contents |
|---|---|
| `users` | Registered accounts (email, hashed password, display name) |
| `rooms` | Room metadata (visibility, turn timer, status, invite code) |
| `games` | Completed game records (room, start/end times) |
| `game_players` | Per-player results (penalty points, rank, winner flag) |

### Redis

Stores transient live game state (`GameState` JSON) with a TTL that is refreshed on every write. Used by the WS server to load, update, and persist game state between moves.

---

## Authentication

Both services share the same `JWT_SECRET`. The JWT payload:

| Claim | Description |
|---|---|
| `sub` | User UUID (random for guests) |
| `display_name` | Player's display name |
| `is_guest` | `true` for guest sessions |
| `exp` | Expiry timestamp |

The WS server validates the JWT on the initial WebSocket upgrade request; unauthenticated connections are rejected immediately.
