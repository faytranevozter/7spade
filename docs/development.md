# Development Guide

## Prerequisites

| Tool | Version |
|---|---|
| Docker + Docker Compose | latest |
| Go | 1.22+ |
| Node.js | 20+ |

---

## Quick Start (Docker)

The fastest way to run the full stack:

```bash
docker compose up --build
```

| Service | URL |
|---|---|
| Web app | http://localhost:3000 |
| HTTP API | http://localhost:8080 |
| WebSocket server | http://localhost:8081 |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |

### Health checks

```bash
curl http://localhost:8080/health   # {"status":"ok","service":"api"}
curl http://localhost:8081/health   # {"status":"ok","service":"ws"}
```

---

## Project Structure

```
7spade/
├── services/
│   ├── api/          # HTTP API: auth, rooms, game history
│   │   ├── cmd/api/  # API entry point
│   │   ├── internal/ # config, database, cache, auth, repositories, handlers, server
│   │   └── Dockerfile
│   └── ws/           # WebSocket game server: real-time gameplay
│       ├── main.go
│       └── Dockerfile
├── web/              # React + TypeScript frontend
│   ├── src/
│   ├── index.html
│   ├── vite.config.ts
│   └── Dockerfile
└── docker-compose.yml
```

---

## Running Services Individually

### HTTP API

```bash
cd services/api
DATABASE_URL=postgres://sevens:sevens@localhost:5432/sevens?sslmode=disable \
REDIS_URL=redis://localhost:6379 \
JWT_SECRET=dev-secret \
go run ./cmd/api
```

From `services/api`, the same command is available as `make run`. Hot reload uses Air via `make dev`, which builds `./cmd/api`.

The API uses a layer-based package layout. The executable is in `cmd/api`, and application packages live under `internal/config`, `internal/database`, `internal/cache`, `internal/auth`, `internal/repository`, `internal/middleware`, `internal/handler`, and `internal/server`.

Database migrations are embedded from `services/api/internal/database/migrations/` and run automatically during API startup.

### WebSocket Server

```bash
cd services/ws
REDIS_URL=redis://localhost:6379 \
JWT_SECRET=dev-secret \
go run .
```

### Frontend

```bash
cd web
npm install
npm run dev
```

The frontend dev server runs at http://localhost:5173 by default (Vite). Update `VITE_API_URL` and `VITE_WS_URL` in `web/.env` to point at the running services.

Frontend UI work uses Tailwind CSS v4.2 through the Vite plugin (`tailwindcss` and `@tailwindcss/vite`). Import Tailwind from the CSS entry with `@import "tailwindcss";`.

Use `design/design_system.html` as the frontend visual source of truth. It defines the Seven Spade palette, DM Sans/DM Mono typography, card states, game-table board layout, avatars, badges, room cards, notifications, score table, spacing, radius, and motion rules.

---

## Environment Variables

Both Go services are configured via environment variables (set in `docker-compose.yml` for local development):

| Variable | Service | Description |
|---|---|---|
| `PORT` | api, ws | HTTP listen port |
| `DATABASE_URL` | api | PostgreSQL connection string |
| `REDIS_URL` | api, ws | Redis connection string |
| `JWT_SECRET` | api, ws | Secret for signing JWTs |
| `FRONTEND_URL` | api | Frontend origin used by OAuth flows |
| `GOOGLE_OAUTH_CLIENT_ID` | api | Google OAuth client ID |
| `GOOGLE_OAUTH_CLIENT_SECRET` | api | Google OAuth client secret |
| `GOOGLE_OAUTH_REDIRECT_URL` | api | Google OAuth callback URL |
| `GITHUB_OAUTH_CLIENT_ID` | api | GitHub OAuth client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | api | GitHub OAuth client secret |
| `GITHUB_OAUTH_REDIRECT_URL` | api | GitHub OAuth callback URL |
| `TELEGRAM_OAUTH_CLIENT_ID` | api | Telegram OIDC client ID from BotFather |
| `TELEGRAM_OAUTH_CLIENT_SECRET` | api | Telegram OIDC client secret from BotFather |
| `TELEGRAM_OAUTH_REDIRECT_URL` | api | Telegram OIDC callback URL |

> **⚠️ Security note:** The `JWT_SECRET` in `docker-compose.yml` is for local development only. Never commit real secrets to source control.

---

## Docker Compose Details

Services start in dependency order:

1. `postgres` and `redis` start first with health checks.
2. `api` waits for both `postgres` and `redis` to be healthy.
3. `ws` waits for `redis` to be healthy.
4. `web` waits for `api` and `ws`.

PostgreSQL credentials for local development:

| Field | Value |
|---|---|
| User | `sevens` |
| Password | `sevens` |
| Database | `sevens` |

---

## Testing

Run Go tests for each service:

```bash
cd services/api && go test ./...
cd services/ws  && go test ./...
```

Equivalent Make targets:

```bash
make -C services/api test
make -C services/ws test
make -C web check
```

The Game State Store tests require a Redis instance (via testcontainers-go or a local Redis).

Verify frontend changes:

```bash
cd web && npm run build && npm run lint
```

Run `cd web && npm test` when a test script exists or when frontend tests are added.
