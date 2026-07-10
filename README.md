# Seven Spade

A real-time multiplayer card game built with Go and React.

## Stack

| Layer | Tech |
|---|---|
| HTTP API | Go (`services/api`) |
| WebSocket game server | Go (`services/ws`) |
| Frontend | React + TypeScript + Vite + Tailwind CSS v4 (`web/`) |
| Database | PostgreSQL 16 |
| OAuth state + live room snapshots + presence | Redis 7 |

> The WebSocket server persists live room state to Redis as room snapshots, so
> games survive a restart. The API uses Redis for OAuth state, email tokens, and
> rate limits. Redis is required by both services. Multi-replica WS may use a
> dedicated `WS_REDIS_URL` for owner/relay coordination.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- Go 1.22+ (for local development)
- Node 20+ (for local frontend development)

## Running locally

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
curl http://localhost:8080/health   # api plus postgres/redis dependency status
curl http://localhost:8081/health   # ws plus postgres/redis dependency status
```

## Project structure

```
7spade/
├── services/
│   ├── api/          # HTTP API: cmd/api + internal packages
│   └── ws/           # WebSocket game server: real-time gameplay
├── web/              # React + Tailwind frontend
├── docs/             # Architecture, API, WebSocket, deployment, specs
└── docker-compose.yml
```

## Environment variables

Both Go services are configured via environment variables (set in `docker-compose.yml`):

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

See [docs/development.md](./docs/development.md#environment-variables) for the
full list, including OAuth provider credentials, SMTP, and frontend `VITE_*` variables.

API migrations are embedded from `services/api/internal/database/migrations/` and applied on startup.

> **Note:** The `JWT_SECRET` in `docker-compose.yml` is for local development only. Never commit real secrets.

## Documentation

Detailed docs live in [`docs/`](./docs/README.md):

- [Game Rules](./docs/game-rules.md)
- [Architecture](./docs/architecture.md)
- [HTTP API Reference](./docs/api.md)
- [WebSocket Protocol](./docs/websocket.md)
- [Development Guide](./docs/development.md)
- [Roadmap](./docs/roadmap.md)
