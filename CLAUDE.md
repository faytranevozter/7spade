# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Seven Spade — a real-time multiplayer card game (4 players, standard 52-card deck). Players build suit sequences from 7s outward; unable-to-play cards go face-down as penalty points. Lowest penalty wins.

## Commands

### Full stack (Docker)
```bash
docker compose up --build          # Run everything
make up-deps                       # Start only postgres + redis (for local dev)
```

### Go services (api, ws)
```bash
cd services/api && make test       # Run tests
cd services/ws && make test        # Run tests
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
- Internal packages: `auth`, `cache`, `config`, `database`, `handler`, `middleware`, `repository`, `server`
- Handles: user auth (guest/register/login + multi-provider OAuth/OIDC), room CRUD, game history
- Migrations embedded from `internal/database/migrations/` and auto-applied on startup
- Internal endpoints called by the WS service (under `/internal/*`): `POST /games`, `POST /rooms/:id/status`, `DELETE /rooms/:id/players/:userId`, `POST /rooms/reconcile`. Guarded by a required `X-Internal-Secret` header (`INTERNAL_API_SECRET`; API fails fast if unset)

**`services/ws`** — WebSocket game server (Go, gorilla/websocket, net/http stdlib)
- Entry: `main.go` (flat package, no `cmd/` nesting)
- Core game logic in `game/` package (engine, bot AI)
- Live `GameState` is held in memory and persisted to **Redis as room snapshots** (`store/`) after every change, so rooms survive a restart (rehydrated lazily on reconnect). Redis is required — the WS service fails fast at startup if it's unreachable
- Manages room lifecycle: lobby phase (ready-up, host starts, bot backfill) → playing phase (turn timer, card moves, rematch voting)
- Calls API internal endpoints to save game results, update room status, and reconcile orphaned rooms

**`web/`** — React SPA (React 19, TypeScript, Vite, Tailwind CSS v4)
- Router: react-router v7
- Key hooks/providers: `AuthProvider` + `useAuth` (sessionStorage token, shared context), `useGameSocket` (WebSocket connection + game state)
- Pages: Auth → Lobby → WaitingRoom → Game → Results, plus History

### Communication Flow

1. Browser authenticates via API, receives JWT
2. Browser connects to WS server at `ws://host/ws?room_id=X&token=JWT`
3. WS server validates JWT, manages room state in-memory
4. On game end, WS POSTs results to API's internal endpoints
5. Frontend reads game history from API

### Data Stores

- **PostgreSQL 16**: Users, OAuth provider links, rooms, room membership, game history (via `services/api`)
- **Redis 7**: OAuth state / PKCE during sign-in (API); live room snapshots so games survive a WS restart (WS, via `store/`); also used for the `/health` dependency checks.

### Game Engine (`services/ws/game/`)

- 4 players always (empty seats filled with bots)
- Suits: spades/hearts/diamonds/clubs; Ranks: 2-14 (Ace=14)
- Aces never extend a sequence — they only close a suit (low after 2, or high after K)
- Ace closing method (high/low) is locked on first use and applies to all suits
- Turn timer auto-plays if player doesn't act

## Environment

Both Go services configured via env vars (see `docker-compose.yml` for defaults). Key vars: `PORT`, `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET`, `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`. The WS service also uses `API_URL` (base URL for internal API calls) and both services share `INTERNAL_API_SECRET` (required guard for `/internal/*`; API fails fast if unset). The API also reads `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` for transactional email (password reset / verification); when `SMTP_HOST` is unset it logs email links to the console instead of sending.

Frontend env: `VITE_API_URL` (defaults to `http://localhost:8080`) and `VITE_WS_URL` (defaults to `ws://localhost:8081`).
