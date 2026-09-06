# Seven Spade

A real-time multiplayer card game built with Go and React.

## Stack

| Layer | Tech |
|---|---|
| HTTP API | Go (`services/api`) |
| WebSocket game server | Go (`services/ws`) |
| Admin API | Go (`services/admin-api`) |
| Player frontend | React + TypeScript + Vite + Tailwind CSS v4 (`web/`) |
| Admin frontend | React + TypeScript + Vite (`admin-web/`) |
| Database | PostgreSQL 16 |
| OAuth state + live room snapshots + presence | Redis 7 |

> The WebSocket server persists live room state to Redis as room snapshots, so
> games survive a restart. The API uses Redis for OAuth state, email tokens, and
> rate limits. Redis is required by both services. Multi-replica WS may use a
> dedicated `WS_REDIS_URL` for owner/relay coordination.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- Go 1.26.1 (for local development)
- Node 24 (recommended; matches CI and the frontend images)

## Running locally

The full Compose stack includes the admin applications. Generate local-only
admin secrets before starting it:

```bash
export ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
export ADMIN_MFA_ENCRYPTION_KEY="$(openssl rand -hex 32)"
docker compose up --build
```

To run only player-facing services, use
`docker compose up --build postgres redis redis-ws api ws web`.

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
curl http://localhost:8080/health   # api plus postgres/redis dependency status
curl http://localhost:8081/health   # ws plus postgres/redis dependency status
```

## Project structure

```
7spade/
├── services/
│   ├── api/          # HTTP API: cmd/api + internal packages
│   ├── ws/           # WebSocket game server: real-time gameplay
│   └── admin-api/    # Separate admin control-plane API
├── web/              # Player frontend
├── admin-web/        # Administrator frontend
├── docs/             # Architecture, API, WebSocket, deployment, specs
├── scripts/          # bump-semver and other repo tooling
├── VERSION           # Semver source of truth (X.Y.Z)
└── docker-compose.yml
```

## Version bump / release

Semver lives in root `VERSION` (and is mirrored in root `package.json`), same pattern as [vibecode-aio](https://github.com/faytranevozter/vibecode-aio).

```bash
make version                 # print current VERSION
make bump-patch              # 0.10.0 → 0.10.1
make bump-minor              # 0.10.0 → 0.11.0
make bump-major              # 0.10.0 → 1.0.0
# or: ./scripts/bump-semver.sh patch|minor|major
```

Then commit, push to `main`, and publish a tag that **must equal** `VERSION` with a `v` prefix:

```bash
git checkout main && git pull
# ensure VERSION is what you want to release
git tag "v$(tr -d '[:space:]' < VERSION)"
git push origin "v$(tr -d '[:space:]' < VERSION)"
```

CI rejects tags that do not match `VERSION`. Publishing a GitHub Release on that tag triggers deploy (build + Swarm rollout).

## Environment variables

The Go services are configured via environment variables (set in `docker-compose.yml`):

| Variable | Service | Description |
|---|---|---|
| `PORT` | api, ws | HTTP listen port |
| `DATABASE_URL` | api, ws | PostgreSQL connection string |
| `REDIS_URL` | api, ws | Redis connection string |
| `WS_REDIS_URL` | ws | Optional dedicated Redis for multi-replica relay |
| `JWT_SECRET` | api, ws | Secret for signing JWTs |
| `API_URL` | ws | HTTP API base URL for internal service calls |
| `INTERNAL_API_SECRET` | api, ws | Required shared secret guarding the API's `/internal/*` endpoints |
| `FRONTEND_URL` | api | Frontend origin used by OAuth and email links |
| `CORS_ALLOWED_ORIGINS` | api | Comma-separated origins allowed for credentialed browser requests |
| `ADMIN_JWT_SECRET` | admin-api | Required, separate administrator access-token signing secret |
| `ADMIN_MFA_ENCRYPTION_KEY` | admin-api | Required persistent, high-entropy secret used to derive the MFA encryption key |
| `WS_INSPECTION_SECRET` / `WS_ADMIN_SERVICE_SECRET` | ws, admin-api | Matching server-only credential for redacted live-room inspection |

See [docs/development.md](./docs/development.md#environment-variables) for the
full list, including OAuth provider credentials, SMTP, and frontend `VITE_*` variables.

API migrations are embedded from `services/api/internal/database/migrations/` and applied on startup.

> **Note:** The `JWT_SECRET` in `docker-compose.yml` is for local development only. Never commit real secrets.

## Documentation

Detailed docs live in [`docs/`](./docs/README.md):

- [Game Rules](./docs/game-rules.md)
- [Architecture](./docs/architecture.md)
- [HTTP API Reference](./docs/api.md)
- [Admin API](./docs/admin-api.md)
- [WebSocket Protocol](./docs/websocket.md)
- [Development Guide](./docs/development.md)
- [Roadmap](./docs/roadmap.md)
