# Development Guide

## Prerequisites

| Tool | Version |
|---|---|
| Docker + Docker Compose | latest |
| Go | 1.26.1 |
| Node.js | 24 recommended (matches CI and Docker builds) |

---

## Quick Start (Docker)

The full stack includes the player and administrator applications. The admin
API refuses to start without its own JWT secret and MFA encryption secret.
Generate high-entropy local-only values in the shell that launches Compose:

```bash
export ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
export ADMIN_MFA_ENCRYPTION_KEY="$(openssl rand -hex 32)"
docker compose up --build
```

Compose interpolation reads these values from the shell or a root `.env` file;
a service-local `.env` file is not used by `docker-compose.yml`. To omit the
admin applications, start the player-facing subset explicitly:

```bash
docker compose up --build postgres redis redis-ws api ws web
```

| Service | URL |
|---|---|
| Web app | http://localhost:3000 |
| HTTP API | http://localhost:8080 |
| WebSocket server | http://localhost:8081 |
| Admin app | http://localhost:3001 |
| Admin API | http://127.0.0.1:8082 |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |
| WS relay Redis | localhost:6380 |

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
│   ├── api/          # HTTP API: auth, rooms, history, stats, friends
│   │   ├── cmd/api/  # API entry point
│   │   ├── internal/ # config, database, cache, auth, email, repository, middleware, handler, server
│   │   └── Dockerfile
│   ├── ws/           # WebSocket game server: real-time gameplay
│   │   ├── main.go       # entry point (flat package)
│   │   ├── config.go     # env loading (JWT, Redis, API_URL, WS_REDIS_URL, …)
│   │   ├── server.go     # connections, room hubs, play loop, rematch
│   │   ├── lobby.go      # lobby phase + internal API clients
│   │   ├── relay_glue.go # multi-replica owner/edge wiring
│   │   ├── game/         # pure game engine + auto-play bot
│   │   ├── store/        # room snapshots + presence
│   │   ├── relay/        # owner lease + pub/sub relay (multi-replica)
│   │   └── Dockerfile
│   └── admin-api/    # Admin auth, RBAC, moderation, events, skins, audit
│       ├── cmd/admin-api/       # service entry point
│       └── cmd/bootstrap-admin/ # first-admin bootstrap command
├── web/              # Player React + TypeScript frontend
│   ├── src/
│   │   ├── pages/    # Auth, Lobby, WaitingRoom, Game, History, Leaderboard, Profile, Watch, Replay, …
│   │   ├── hooks/    # useAuth, useGameSocket, useSpectatorSocket, …
│   │   └── api/      # HTTP client modules
│   ├── index.html
│   ├── vite.config.ts
│   └── Dockerfile
├── admin-web/        # Administrator React + TypeScript frontend
├── docs/             # Architecture, API, WebSocket, deployment, specs
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
INTERNAL_API_SECRET=dev-internal-secret \
go run ./cmd/api
```

From `services/api`, the same command is available as `make run`. Hot reload uses Air via `make dev`, which builds `./cmd/api`.

The API uses a layer-based package layout. The executable is in `cmd/api`, and application packages live under `internal/config`, `internal/database`, `internal/cache`, `internal/auth`, `internal/repository`, `internal/middleware`, `internal/handler`, and `internal/server`.

Database migrations are embedded from `services/api/internal/database/migrations/` and run automatically during API startup.

When API routes or `docs/openapi.yaml` change, run `make validate-openapi` from the repository root. It checks OpenAPI YAML parsing, local `$ref` resolution, formatting, and method/path parity with `services/api/internal/server/router.go`. CI runs the same validation for changes under `docs/**` or `services/api/**`.

### WebSocket Server

```bash
cd services/ws
DATABASE_URL=postgres://sevens:sevens@localhost:5432/sevens?sslmode=disable \
REDIS_URL=redis://localhost:6379 \
JWT_SECRET=dev-secret \
API_URL=http://localhost:8080 \
INTERNAL_API_SECRET=dev-internal-secret \
go run .
```

`API_URL` enables the WS server's calls to the API's internal endpoints
(game-history persistence, room-status updates, member removal, orphan-room
reconcile). When it is empty, those calls are skipped. `INTERNAL_API_SECRET` is
**required** on the API (startup fails if unset) and must match on the WS
service so `/internal/*` calls authenticate. `make dev` (Air hot-reload) is also
available from `services/ws`.

`REDIS_URL` is **required** by the WS server: it persists live room snapshots to
Redis (so rooms survive a restart) and exits at startup if Redis is unreachable.

### Frontend

```bash
cd web
npm install
npm run dev
```

The frontend dev server runs at http://localhost:5173 by default (Vite). Update `VITE_API_URL` and `VITE_WS_URL` in `web/.env` to point at the running services.

The API CORS middleware allows credentialed browser requests from origins listed in `CORS_ALLOWED_ORIGINS`. This is required because refresh tokens are transported by HttpOnly cookies and browser credentialed requests cannot use `Access-Control-Allow-Origin: *`.

Frontend UI work uses Tailwind CSS v4.2 through the Vite plugin (`tailwindcss` and `@tailwindcss/vite`). Import Tailwind from the CSS entry with `@import "tailwindcss";`.

Use `design/design_system.html` as the frontend visual source of truth. It defines the Seven Spade palette, DM Sans/DM Mono typography, card states, game-table board layout, avatars, badges, room cards, notifications, score table, spacing, radius, and motion rules.

### Admin Applications

Run the admin API after the player API has applied the shared database schema:

```bash
cd services/admin-api
cp .env.example .env
# Set DATABASE_URL, ADMIN_JWT_SECRET, and ADMIN_MFA_ENCRYPTION_KEY.
make run
```

Bootstrap the first administrator once, using the same database and encryption
key as the admin API:

```bash
ADMIN_BOOTSTRAP_EMAIL=admin@example.test \
ADMIN_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
ADMIN_BOOTSTRAP_NAME='Local Admin' \
make bootstrap
```

Run `npm install && npm run dev` from `admin-web/`; its Vite server uses
http://localhost:5174. In Compose, nginx serves it at http://localhost:3001.
Keep `ADMIN_FRONTEND_ORIGIN` equal to the browser origin exactly. Set
`ADMIN_SECURE_COOKIES=true` only behind HTTPS.

---

## Environment Variables

The Go services are configured via environment variables (set in `docker-compose.yml` for local development):

| Variable | Service | Description |
|---|---|---|
| `PORT` | api, ws | HTTP listen port |
| `DATABASE_URL` | api, ws | PostgreSQL connection string (ws uses it for the health check only) |
| `REDIS_URL` | api, ws | Redis connection string. **Required by both** — the WS service persists live room snapshots to Redis and fails fast at startup if it is unreachable |
| `WS_REDIS_URL` | ws | Optional dedicated Redis for multi-replica owner/relay. Falls back to `REDIS_URL` when unset (single-replica local dev) |
| `JWT_SECRET` | api, ws | Secret for signing JWTs (must match across both services) |
| `API_URL` | ws | Base URL of the HTTP API for internal calls; internal calls are skipped if empty |
| `INTERNAL_API_SECRET` | api, ws | **Required** shared secret guarding the API's `/internal/*` endpoints; API fails fast if unset; must match on both services |
| `FRONTEND_URL` | api | Frontend origin used by OAuth flows and email deep links |
| `CORS_ALLOWED_ORIGINS` | api | Comma-separated origins allowed for credentialed browser requests |
| `LEADERBOARD_MIN_GAMES` | api | Min games to appear on leaderboard (default `5`) |
| `RATE_LIMIT_AUTH_PER_MINUTE` | api | Auth endpoints (login/register/guest/OAuth/…) per IP (default `10`) |
| `RATE_LIMIT_ROOMS_WRITE_PER_MINUTE` | api | Room create/join per user (default `5`) |
| `RATE_LIMIT_SOCIAL_PER_MINUTE` | api | Friends/search mutations per user (default `30`) |
| `RATE_LIMIT_GENERAL_PER_MINUTE` | api | General API reads per identity (default `60`) |
| `RATE_LIMIT_WINDOW_SECONDS` | api | Fixed window for the tiers above (default `60`) |
| `RATE_LIMIT_QUICK_PLAY_COOLDOWN_MS` | api | Quick-play cooldown per user (default `3000`) |
| `SMTP_HOST` | api | SMTP host; when unset, password-reset / verification links are logged to the console |
| `SMTP_PORT` | api | Default `587` |
| `SMTP_USER` / `SMTP_PASS` | api | SMTP credentials |
| `SMTP_FROM` / `SMTP_FROM_NAME` / `SMTP_REPLY_TO` | api | From / display name / Reply-To |
| `SMTP_ENCRYPTION` | api | `auto`, `tls`, `starttls`, or `none` |
| `GOOGLE_OAUTH_CLIENT_ID` | api | Google OAuth client ID |
| `GOOGLE_OAUTH_CLIENT_SECRET` | api | Google OAuth client secret |
| `GOOGLE_OAUTH_REDIRECT_URL` | api | Google OAuth callback URL |
| `GITHUB_OAUTH_CLIENT_ID` | api | GitHub OAuth client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | api | GitHub OAuth client secret |
| `GITHUB_OAUTH_REDIRECT_URL` | api | GitHub OAuth callback URL |
| `TELEGRAM_OAUTH_CLIENT_ID` | api | Telegram OIDC client ID from BotFather |
| `TELEGRAM_OAUTH_CLIENT_SECRET` | api | Telegram OIDC client secret from BotFather |
| `TELEGRAM_OAUTH_REDIRECT_URL` | api | Telegram OIDC callback URL |
| `TELEGRAM_MOBILE_REDIRECT_URL` | api | Telegram mobile HTTPS callback URL |
| `S3_ENDPOINT` / `S3_BUCKET` / `S3_REGION` | api, admin-api | Optional S3-compatible skin-asset storage; missing storage degrades asset upload rather than blocking API startup |
| `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | api, admin-api | Server-side upload credentials; never expose them to a frontend |
| `S3_PUBLIC_URL` | api, admin-api | Public CDN or bucket URL used in rendered asset URLs |
| `ADMIN_JWT_SECRET` | admin-api | Required signing secret dedicated to admin access tokens; do not reuse `JWT_SECRET` |
| `ADMIN_MFA_ENCRYPTION_KEY` | admin-api | Required persistent, high-entropy secret used to derive the MFA encryption key; losing it invalidates encrypted MFA enrollments |
| `ADMIN_FRONTEND_ORIGIN` | admin-api | Exact allowed credentialed browser origin |
| `WS_ADMIN_URL` / `WS_ADMIN_SERVICE_SECRET` | admin-api | Optional live-room inspection endpoint and server credential; the secret must match WS `WS_INSPECTION_SECRET` |
| `OPERATIONS_*_URL` | admin-api | Metrics, logs, traces, deployments, and runbook links; all five are required when `APP_ENV=production` |

The frontend reads `VITE_API_URL` (default `http://localhost:8080`) and
`VITE_WS_URL` (default `ws://localhost:8081`) from `web/.env`.

See [deployment/environment.md](./deployment/environment.md) for production env files.

> **⚠️ Security note:** The `JWT_SECRET` in `docker-compose.yml` is for local development only. Never commit real secrets to source control.

---

## Docker Compose Details

Services start in dependency order:

1. `postgres` and `redis` start first with health checks.
2. `api` waits for both `postgres` and `redis` to be healthy.
3. `ws` waits for both `postgres` and `redis` to be healthy.
4. `admin-api` waits for PostgreSQL and the player API migrations.
5. `web` waits for `api` and `ws`; `admin-web` waits for `admin-api`.

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
cd services/admin-api && go test ./...
```

Equivalent Make targets:

```bash
make -C services/api test
make -C services/ws test
make -C services/admin-api test
```

The `services/ws/store` package's room-snapshot tests run against an in-process
`miniredis` server, so they need no external Redis to run. Relay and presence
tests similarly use in-process Redis where needed.

Verify frontend changes:

```bash
cd web && npm test          # Vitest unit tests
cd web && npm run test:e2e  # Playwright end-to-end tests
cd web && npm run lint      # ESLint
cd web && npm run build     # TypeScript check + Vite build
cd admin-web && npm test
cd admin-web && npm run lint
cd admin-web && npm run build
```

From the repository root, `make test`, `make lint`, and `make build` cover all
three Go services and both frontends. `make -C web check` runs build and lint,
not unit or end-to-end tests.
