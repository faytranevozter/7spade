# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Seven Spade — a real-time multiplayer card game. Classic default is 4 players
and a 52-card deck; custom rooms support 2–8 seats, double deck, teams, and
alternate scoring. Players build suit sequences from 7s outward; unable-to-play
cards go face-down as penalty points. Lowest penalty wins.

## Commands

### Full stack (Docker)
```bash
export ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
export ADMIN_MFA_ENCRYPTION_KEY="$(openssl rand -hex 32)"
docker compose up --build          # Run player + admin applications
make up-deps                       # Start only postgres + redis (for local dev)
```

### Go services (api, ws, admin-api)
```bash
cd services/api && make test       # Run tests
cd services/ws && make test        # Run tests
cd services/admin-api && make test # Run tests
cd services/api && make dev        # Hot-reload (requires air)
cd services/ws && make dev         # Hot-reload (requires air)
cd services/api && go test -v -run TestName ./...  # Single test
```

### Web frontend
```bash
cd web && npm run dev              # Vite dev server (port 5173)
cd web && npm test                 # Vitest unit tests
cd web && npm run test:e2e         # Playwright e2e tests
cd web && npm run lint             # ESLint
cd web && npm run build            # TypeScript check + Vite build
cd admin-web && npm run dev        # Admin Vite dev server (port 5174)
```

### Root-level shortcuts
```bash
make test                          # Test all services + web
make lint                          # Lint all
make dev                           # Hot-reload all services + frontend
```

## Architecture

### Services

**`services/api`** — HTTP REST API (Go, Gin framework)
- Entry: `cmd/api/main.go`
- Internal packages: `auth`, `cache`, `config`, `database`, `email`, `handler`, `middleware`, `repository`, `server`
- Handles: user auth (guest/register/login + multi-provider OAuth/OIDC, password reset, email verification), room CRUD / quick-play, game history + replay, stats / leaderboard / seasons / achievements / rating history, friends + search
- Migrations embedded from `internal/database/migrations/` and auto-applied on startup
- Internal endpoints called by the WS service (under `/internal/*`): `POST /games`, `POST /rooms/:id/status`, `DELETE /rooms/:id/players/:userId`, `POST /rooms/:id/kick/:userId`, `POST /rooms/reconcile`. Guarded by a required `X-Internal-Secret` header (`INTERNAL_API_SECRET`; API fails fast if unset)

**`services/ws`** — WebSocket game server (Go, gorilla/websocket, net/http stdlib)
- Entry: `cmd/ws/main.go`; dependency assembly in `internal/app`
- Focused runtime modules: `internal/session`, `transport`, `room`, `cluster`, `persistence`, `apiclient`, and `httpserver`
- Core game logic in `game/` package (engine, bot AI, `GameConfig` for custom modes)
- Live `GameState` is held in memory and persisted to **Redis as room snapshots** (`store/`) after every change, so rooms survive a restart (rehydrated lazily on reconnect). Redis is required — the WS service fails fast at startup if it's unreachable
- Friends presence written to Redis (`store/presence.go`)
- Multi-replica: optional `WS_REDIS_URL` + `relay/` owner lease and pub/sub edge path
- Manages room lifecycle: lobby (ready-up, host start/kick, team pick, bot backfill) → playing (turn timer, card moves, emotes) → rematch (countdown, partial return-to-lobby) or waiting room
- Spectator join via `?role=spectator` (redacted `spectator_state`)
- Calls API internal endpoints to save game results (rating/XP deltas), update room status, kick, and reconcile orphaned rooms

**`services/admin-api`** — separate administrator API (Go, Gin)
- Entry: `cmd/admin-api/main.go`; first-admin bootstrap: `cmd/bootstrap-admin`
- Separate admin JWT/refresh sessions, MFA, RBAC, moderation, investigations, events, achievements, revisioned skins, and audit export
- Optional redacted live-room inspection uses a server-only credential shared with WS

**`web/`** — React SPA (React 19, TypeScript, Vite, Tailwind CSS v4)
- Router: react-router v7
- Key hooks/providers: `AuthProvider` + `useAuth` (sessionStorage token, shared context), `useGameSocket` (player WebSocket + game state), `useSpectatorSocket` (watch mode)
- Pages: Auth / Register / OAuth callbacks / password-reset / verify-email → Lobby → WaitingRoom → Game → Results; History, Leaderboard, My Profile (`/me`), public Profile (`/players/:id`), Watch (`/watch/:roomId`), Replay (`/replay/:gameId`)

**`admin-web/`** — administrator React SPA with its own API client and auth flow

### Communication Flow

1. Browser authenticates via API, receives JWT
2. Browser connects to WS server at `ws://host/ws?room_id=X&token=JWT` (add `&role=spectator` to watch)
3. WS server validates JWT, manages room state in-memory (+ Redis snapshot)
4. On game end, WS POSTs results to API's internal endpoints; clients get `game_over` with optional rating/XP fields
5. Frontend reads history, stats, friends, and replays from API

### Data Stores

- **PostgreSQL 16**: Users, OAuth links, rooms/membership/kicks, game history + replays, stats/seasons/ratings/XP, achievements, friendships (via `services/api`)
- **Redis 7**: OAuth state / PKCE / email tokens / rate limits (API); live room snapshots + presence (WS); optional dedicated `WS_REDIS_URL` for multi-replica relay

### Game Engine (`services/ws/game/`)

- Default 4 players FFA; configurable via `GameConfig` (2–8 seats, deck count, scoring, 2v2)
- Empty seats filled with bots at start; practice mode allows solo start
- Suits: spades/hearts/diamonds/clubs; Ranks: 2-14 (Ace=14)
- Aces never extend a sequence — they only close a suit (low after 2, or high after K)
- Ace closing method (high/low) is locked on first use and applies to all suits
- Stalemate early-end when no player has a legal play
- Turn timer auto-plays if player doesn't act

## Environment

The Go services are configured via env vars (see `docker-compose.yml`). Key player-service vars: `PORT`, `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET`, `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`. The WS service also uses `API_URL` and optional `WS_REDIS_URL`. API and WS share required `INTERNAL_API_SECRET`. The API also reads SMTP vars. S3-compatible asset storage and the `cmd/skinassets` uploader belong to `services/admin-api`.

Admin API env: `ADMIN_JWT_SECRET` and `ADMIN_MFA_ENCRYPTION_KEY` are required; the latter encrypts TOTP secrets at rest. `ADMIN_FRONTEND_ORIGIN`, `ADMIN_SECURE_COOKIES`, and `APP_ENV` control browser and production policy behavior.

Frontend env: `VITE_API_URL` (defaults to `http://localhost:8080`) and `VITE_WS_URL` (defaults to `ws://localhost:8081`).

Full docs: [`docs/`](./docs/README.md).

## Agent skills

### Issue tracker

Issues and specs live in GitHub Issues; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the five canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Domain documentation is optional and has not been initialized in this repo. If
a change needs durable domain vocabulary or an architectural decision, follow
`docs/agents/domain.md` to create `CONTEXT.md` or an ADR deliberately.
