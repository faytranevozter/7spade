# Seven Spade

A real-time multiplayer card game built with Go and React.

## Stack

| Layer | Tech |
|---|---|
| HTTP API | Go (`services/api`) |
| WebSocket game server | Go (`services/ws`) |
| Frontend | React + TypeScript + Vite + Tailwind CSS v4.2 (`web/`) |
| Database | PostgreSQL 16 |
| Live game state | Redis 7 |

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
│   ├── api/          # HTTP API: auth, rooms, game history
│   └── ws/           # WebSocket game server: real-time gameplay
├── web/              # React + Tailwind frontend
└── docker-compose.yml
```

## Environment variables

Both Go services are configured via environment variables (set in `docker-compose.yml`):

| Variable | Service | Description |
|---|---|---|
| `PORT` | api, ws | HTTP listen port |
| `DATABASE_URL` | api, ws | PostgreSQL connection string |
| `REDIS_URL` | api, ws | Redis connection string |
| `JWT_SECRET` | api, ws | Secret for signing JWTs |

> **Note:** The `JWT_SECRET` in `docker-compose.yml` is for local development only. Never commit real secrets.
