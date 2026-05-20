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
- Internal endpoints (`/internal/games`, `/internal/rooms/:id/status`) called by the WS service

**`services/ws`** — WebSocket game server (Go, gorilla/websocket, net/http stdlib)
- Entry: `main.go` (flat package, no `cmd/` nesting)
- Core game logic in `game/` package (engine, bot AI)
- State persistence in `store/` (Redis-backed)
- Manages room lifecycle: lobby phase (ready-up, host starts, bot backfill) → playing phase (turn timer, card moves, rematch voting)
- Calls API internal endpoints to save game results and update room status

**`web/`** — React SPA (React 19, TypeScript, Vite, Tailwind CSS v4)
- Router: react-router v7
- Key hooks: `useAuth` (sessionStorage token management), `useGameSocket` (WebSocket connection + game state)
- Pages: Auth → Lobby → WaitingRoom → Game → Results, plus History

### Communication Flow

1. Browser authenticates via API, receives JWT
2. Browser connects to WS server at `ws://host/ws?room_id=X&token=JWT`
3. WS server validates JWT, manages room state in-memory
4. On game end, WS POSTs results to API's internal endpoints
5. Frontend reads game history from API

### Data Stores

- **PostgreSQL 16**: Users, rooms, game history, OAuth state (via `services/api`)
- **Redis 7**: Session/state caching, used by both services

### Game Engine (`services/ws/game/`)

- 4 players always (empty seats filled with bots)
- Suits: spades/hearts/diamonds/clubs; Ranks: 2-14 (Ace=14)
- Ace closing method (high/low) is locked on first use and applies to all suits
- Turn timer auto-plays if player doesn't act

## Environment

Both Go services configured via env vars (see `docker-compose.yml` for defaults). Key vars: `PORT`, `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET`, `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`.

Frontend env: `VITE_WS_URL` (defaults to `ws://localhost:8081`).
