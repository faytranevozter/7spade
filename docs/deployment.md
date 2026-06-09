# Deployment Guide

This guide covers deploying Seven Spade to production on a single VPS using Docker Swarm (`docker stack deploy`) behind an nginx reverse proxy with TLS.

> **Swarm vs Compose:** the same `docker-compose.yml` is used as the stack file, but Swarm **ignores `build:` and `depends_on`**. You build the service images first (locally on a single node, or via a registry for multi-node) and reference them with `image:` tags; Swarm then schedules them and restarts unhealthy tasks until dependencies come up. Operational commands are `docker stack ...` / `docker service ...` rather than `docker compose ...`.

The deployed stack includes the full feature set: guest/email/OAuth auth (Google, GitHub, Telegram), password reset + email verification, real-time gameplay with bot backfill and difficulty levels, practice mode, game history, achievements, friends + fuzzy player search, profile stat comparison, and seasonal leaderboards with ELO ratings. All of these run on the same five containers below — most are toggled purely by environment variables (e.g. OAuth providers, SMTP), with no extra services required.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Production Topology](#production-topology)
- [Environment Variables](#environment-variables)
- [Step-by-Step Deployment](#step-by-step-deployment)
- [Reverse Proxy Configuration](#reverse-proxy-configuration)
- [TLS with Certbot](#tls-with-certbot)
- [fahrur.my.id Setup](#fahrurmyid-setup)
- [Health Checks](#health-checks)
- [Backups](#backups)
- [Monitoring](#monitoring)
- [CI/CD (GitHub Actions)](#cicd-github-actions)
- [Upgrading](#upgrading)
- [Scaling Notes](#scaling-notes)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Requirement | Minimum |
|---|---|
| VPS CPU | 1 vCPU |
| RAM | 2 GB (1 GB works, 2 GB recommended for build spikes) |
| Disk | 20 GB SSD |
| OS | Ubuntu 22.04 LTS or Debian 12+ |
| Docker | 24+ with Swarm mode enabled |
| Domain | One domain with three A records pointing to the VPS |
| Ports | 80, 443 open to the internet |

Install Docker and initialize Swarm on a fresh Ubuntu/Debian server:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# log out and back in to apply group change
docker swarm init          # enable Swarm mode (single-node manager)
docker node ls             # verify the node is Ready / active
```

> `docker swarm init` on a single VPS makes it a one-node manager — enough to run `docker stack deploy`. For a multi-node cluster, `docker swarm join` additional workers with the token printed by `init`.

---

## Production Topology

```
                          Internet
                             │
                     ┌───────▼────────┐
                     │   nginx (443)  │  TLS termination
                     └───┬───┬───┬────┘
                         │   │   │
           ┌─────────────┘   │   └─────────────┐
           │                 │                 │
   ┌───────▼──────┐  ┌──────▼───────┐  ┌──────▼───────┐
   │   web :80    │  │  api :8080   │  │  ws  :8081   │
   │  (nginx SPA) │  │   Go HTTP    │  │ Go WebSocket │
   └──────────────┘  └──────┬───────┘  └──────┬───────┘
                            │                  │
                    ┌───────▼──────┐   ┌───────▼──────┐
                    │ PostgreSQL 16│   │   Redis 7    │
                    └──────────────┘   └──────────────┘
```

Three subdomains are used in production:

| Subdomain | Service | Port |
|---|---|---|
| `spade.example.com` | Web frontend | 80 (internal nginx) |
| `api-spade.example.com` | HTTP API | 8080 |
| `wsspade.example.com` | WebSocket server | 8081 |

---

## Environment Variables

Create `.env` files in each service directory before building. Copy from `.env.example` and fill in production values.

### `services/api/.env`

| Variable | Required | Example (production) |
|---|---|---|
| `PORT` | Yes | `8080` |
| `DATABASE_URL` | Yes | `postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable` |
| `REDIS_URL` | Yes | `redis://redis:6379` |
| `JWT_SECRET` | Yes | `<32+ char random string>` |
| `OAUTH_STATE_SECRET` | No | Falls back to `JWT_SECRET` if empty |
| `INTERNAL_API_SECRET` | Yes | `<shared secret matching ws service>` |
| `FRONTEND_URL` | Yes | `https://spade.example.com` |
| `CORS_ALLOWED_ORIGINS` | Yes | `https://spade.example.com,https://api-spade.example.com` |
| `LEADERBOARD_MIN_GAMES` | No | Min games to qualify for the leaderboard (default `5`) |
| `SMTP_HOST` | No | SMTP server host for password-reset / email-verification mail. When unset, the API logs the links to stdout (dev mode) instead of sending |
| `SMTP_PORT` | No | SMTP port (default `587`) |
| `SMTP_USER` | No | SMTP username |
| `SMTP_PASS` | No | SMTP password |
| `SMTP_FROM` | No | From address (default `no-reply@sevenspade.local`) |
| `GOOGLE_OAUTH_CLIENT_ID` | Optional | Google OAuth client ID |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Optional | Google OAuth client secret |
| `GOOGLE_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/google` |
| `GITHUB_OAUTH_CLIENT_ID` | Optional | GitHub OAuth client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | Optional | GitHub OAuth client secret |
| `GITHUB_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/github` |
| `TELEGRAM_OAUTH_CLIENT_ID` | Optional | Telegram OIDC client ID |
| `TELEGRAM_OAUTH_CLIENT_SECRET` | Optional | Telegram OIDC client secret |
| `TELEGRAM_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/telegram` |

### `services/ws/.env`

| Variable | Required | Example (production) |
|---|---|---|
| `PORT` | Yes | `8081` |
| `DATABASE_URL` | Yes | `postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable` |
| `REDIS_URL` | Yes | `redis://redis:6379` |
| `JWT_SECRET` | Yes | `<must match api JWT_SECRET>` |
| `API_URL` | Yes | `http://api:8080` |
| `INTERNAL_API_SECRET` | Yes | `<must match api INTERNAL_API_SECRET>` |

### `web/.env`

| Variable | Required | Example (production) |
|---|---|---|
| `VITE_API_URL` | Yes | `https://api-spade.example.com` |
| `VITE_WS_URL` | Yes | `wss://wsspade.example.com` |
| `VITE_WS_HEALTH_URL` | Yes | `https://wsspade.example.com` |

### `mobile/.env`

| Variable | Required | Example (production) |
|---|---|---|
| `EXPO_PUBLIC_API_URL` | Yes | `https://api-spade.example.com` |
| `EXPO_PUBLIC_WS_URL` | Yes | `wss://wsspade.example.com` |

> **Security:** Generate a strong `JWT_SECRET` with `openssl rand -base64 32`. The API and WS services must share the same value. `INTERNAL_API_SECRET` must also match across both services. In Swarm you can keep these out of the stack file with `docker secret create jwt_secret -` and a `secrets:` block on each service (mounted at `/run/secrets/<name>`); for a single-node deploy, inline `environment:` values are acceptable if the VPS is trusted.

---

## Step-by-Step Deployment

### 1. Provision the VPS and DNS

Create three DNS A records pointing to your VPS IP:

```
spade.example.com       → <VPS IP>
api-spade.example.com   → <VPS IP>
wsspade.example.com     → <VPS IP>
```

### 2. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 3. Clone the repository

```bash
git clone https://github.com/<owner>/7spade.git /opt/7spade
cd /opt/7spade
```

### 4. Configure environment variables

Copy `.env.example` to `.env` in each service directory and fill in production values:

```bash
cp services/api/.env.example services/api/.env
cp services/ws/.env.example services/ws/.env
cp web/.env.example web/.env
```

Edit each file with the production values from the [Environment Variables](#environment-variables) table above.

### 5. Prepare the stack file for Swarm

Swarm ignores `build:` and `depends_on`, so each service needs an `image:` tag and (optionally) a `deploy:` block. Two approaches:

**Single-node (build locally, no registry):** build the images on the VPS with the same tags the stack file references, then deploy. Add an `image:` to each app service in `docker-compose.yml` (keeping `build:` for the local build step — `docker compose build` reads it, `docker stack deploy` ignores it):

```yaml
  api:
    build: ./services/api
    image: 7spade/api:latest      # add this
  ws:
    build: ./services/ws
    image: 7spade/ws:latest       # add this
  web:
    build: ./web
    image: 7spade/web:latest      # add this
```

Replace each `depends_on:` condition block with a Swarm restart policy so tasks retry until Postgres/Redis are reachable (Swarm has no `condition: service_healthy` gate):

```yaml
    deploy:
      restart_policy:
        condition: on-failure
```

**Multi-node (registry):** push images to a registry (see [CI/CD Option B](#cicd-github-actions)) and set `image: ghcr.io/<owner>/7spade/<service>:<tag>` so every node can pull them.

### 6. Build the images and deploy the stack

Build the images locally (Swarm won't build for you):

```bash
docker compose build          # builds api/ws/web from their build: contexts
```

Deploy the stack (the compose file doubles as the stack file):

```bash
docker stack deploy -c docker-compose.yml 7spade
```

Verify all services are running (replicas converge to `1/1` once images pull and health checks pass):

```bash
docker stack services 7spade
```

Expected output: 5 services, each `1/1`.

> Swarm doesn't read `.env` files referenced by `env_file` the way Compose does at deploy time on remote nodes — for a single-node deploy the `environment:`/`env_file` in the stack file is read at deploy time on the manager, which is fine here. For secrets, prefer `docker secret` (see the security note in [Environment Variables](#environment-variables)).

### 7. Install nginx (reverse proxy)

```bash
sudo apt update
sudo apt install -y nginx
sudo systemctl enable nginx
```

### 8. Configure nginx reverse proxy

See [Reverse Proxy Configuration](#reverse-proxy-configuration) below.

### 9. Enable TLS

See [TLS with Certbot](#tls-with-certbot) below.

---

## Reverse Proxy Configuration

Create `/etc/nginx/sites-available/7spade` and symlink to `sites-enabled`:

```nginx
upstream web {
    server 127.0.0.1:3000;
}
upstream api {
    server 127.0.0.1:8080;
}
upstream ws {
    server 127.0.0.1:8081;
}

# ── Web frontend ────────────────────────────────────────
server {
    listen 80;
    server_name spade.example.com;

    location / {
        proxy_pass http://web;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# ── HTTP API ────────────────────────────────────────────
server {
    listen 80;
    server_name api-spade.example.com;

    client_max_body_size 10m;

    location / {
        proxy_pass http://api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# ── WebSocket server ────────────────────────────────────
server {
    listen 80;
    server_name wsspade.example.com;

    location / {
        proxy_pass http://ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket timeouts (adjust to match turn timer)
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

Enable the site:

```bash
sudo ln -s /etc/nginx/sites-available/7spade /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

> **Key detail:** The WebSocket `server` block requires `proxy_http_version 1.1` and the `Upgrade` / `Connection` headers — without them the WS handshake fails silently.

---

## TLS with Certbot

Install certbot and the nginx plugin:

```bash
sudo apt install -y certbot python3-certbot-nginx
```

Obtain certificates for all three subdomains:

```bash
sudo certbot --nginx \
  -d spade.example.com \
  -d api-spade.example.com \
  -d wsspade.example.com
```

Certbot will:
1. Redirect HTTP → HTTPS automatically
2. Add the `ssl_certificate` directives to each server block
3. Register a renewal cron job (`/etc/cron.d/certbot`)

Verify auto-renewal:

```bash
sudo certbot renew --dry-run
```

---

## fahrur.my.id Setup

This is the concrete production configuration currently in use.

### DNS records

| Subdomain | Type | Value |
|---|---|---|
| `spade.fahrur.my.id` | A | `<VPS IP>` |
| `api-spade.fahrur.my.id` | A | `<VPS IP>` |
| `wsspade.fahrur.my.id` | A | `<VPS IP>` |

### Production environment values

**`services/api/.env`**

```env
PORT=8080
DATABASE_URL=postgres://sevens:<REDACTED>@postgres:5432/sevens?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=<REDACTED>
INTERNAL_API_SECRET=<REDACTED>
FRONTEND_URL=https://spade.fahrur.my.id
CORS_ALLOWED_ORIGINS=https://spade.fahrur.my.id,https://api-spade.fahrur.my.id
OAUTH_STATE_SECRET=<REDACTED>
GOOGLE_OAUTH_CLIENT_ID=<REDACTED>
GOOGLE_OAUTH_CLIENT_SECRET=<REDACTED>
GOOGLE_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/google
GITHUB_OAUTH_CLIENT_ID=<REDACTED>
GITHUB_OAUTH_CLIENT_SECRET=<REDACTED>
GITHUB_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/github
TELEGRAM_OAUTH_CLIENT_ID=<REDACTED>
TELEGRAM_OAUTH_CLIENT_SECRET=<REDACTED>
TELEGRAM_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/telegram
```

**`services/ws/.env`**

```env
PORT=8081
DATABASE_URL=postgres://sevens:<REDACTED>@postgres:5432/sevens?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=<REDACTED>
API_URL=http://api:8080
INTERNAL_API_SECRET=<REDACTED>
```

**`web/.env`**

```env
VITE_API_URL=https://api-spade.fahrur.my.id
VITE_WS_URL=wss://wsspade.fahrur.my.id
VITE_WS_HEALTH_URL=https://wsspade.fahrur.my.id
```

**`mobile/.env`**

```env
EXPO_PUBLIC_API_URL=https://api-spade.fahrur.my.id
EXPO_PUBLIC_WS_URL=wss://wsspade.fahrur.my.id
```

> Replace `<REDACTED>` with real values before deploying. Store secrets outside the repo (use a secrets manager or environment-only injection).

> **Optional vars not shown above:** the production API does not set `SMTP_*`, so password-reset / email-verification links are logged to the API container's stdout rather than emailed — set `SMTP_HOST` (and friends) to send real mail. `LEADERBOARD_MIN_GAMES` is left at its default of `5`.

---

## Health Checks

Both services expose `/health` endpoints. Use them to verify the stack after deploy:

```bash
# API service + its dependencies (PostgreSQL, Redis)
curl -s https://api-spade.example.com/health | jq

# WS service + its dependencies
curl -s https://wsspade.example.com/health | jq
```

Expected response:

```json
{"status":"ok","service":"api"}
```

Also check Swarm task health:

```bash
docker stack services 7spade
# Each service should show REPLICAS 1/1
docker stack ps 7spade --no-trunc
# Per-task state; CURRENT STATE should be "Running" (look for failed/restarting tasks)
```

Add these as uptime monitor targets (e.g., UptimeRobot, Better Stack, or a simple cron):

```bash
# Cron: check every 5 minutes, alert if down
*/5 * * * * curl -sf https://api-spade.example.com/health >/dev/null || echo "API DOWN" | mail -s "7spade alert" ops@example.com
```

---

## Backups

### PostgreSQL

Schedule a daily `pg_dump`. Swarm has no `exec` subcommand, so resolve the running task's container id and `docker exec` into it:

```bash
# Add to crontab: backup every day at 3am UTC
0 3 * * * cid=$(docker ps -q -f name=7spade_postgres) && docker exec -i "$cid" pg_dump -U sevens sevens | gzip > /opt/backups/sevens-$(date +\%Y\%m\%d).sql.gz
```

Rotate old backups (keep last 30 days):

```bash
find /opt/backups -name "sevens-*.sql.gz" -mtime +30 -delete
```

Restore:

```bash
cid=$(docker ps -q -f name=7spade_postgres)
gunzip -c /opt/backups/sevens-20250101.sql.gz | docker exec -i "$cid" psql -U sevens sevens
```

### Redis

Redis is used for:
- **OAuth state** (API) — 10-minute TTL, transient, no backup needed
- **Room snapshots** (WS) — 1-hour TTL by default, rebuilt on next game

Redis data is not durable-critical. If Redis is lost, in-progress rooms will be reset to disconnected state and rehydrate on next player reconnect. No backup cron required.

---

## Monitoring

### Logs

```bash
# All services in the stack
docker stack services 7spade

# Follow a single service's logs (aggregated across its tasks)
docker service logs -f 7spade_api
docker service logs -f 7spade_ws
```

### Resource usage

```bash
docker stats --no-stream
```

### Database connections

```bash
cid=$(docker ps -q -f name=7spade_postgres)
docker exec -i "$cid" psql -U sevens -c "SELECT count(*) FROM pg_stat_activity WHERE datname='sevens';"
```

### Suggested external monitoring

| Tool | Purpose |
|---|---|
| UptimeRobot / Better Stack | HTTP health endpoint checks |
| Grafana + Prometheus | Metrics if scaling beyond single VPS |
| Sentry | Frontend + backend error tracking |

---

## CI/CD (GitHub Actions)

No CI/CD pipeline is currently configured. Below is a recommended template for when one is added.

### Option A: Build on server (simplest)

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: SSH and deploy
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.VPS_HOST }}
          username: ${{ secrets.VPS_USER }}
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            cd /opt/7spade
            git pull origin main
            docker compose build
            docker stack deploy -c docker-compose.yml 7spade
```

### Option B: Build and push images (recommended for scale)

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        service: [api, ws, web]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v5
        with:
          context: ./services/${{ matrix.service }}  # adjust path for web
          push: true
          tags: ghcr.io/${{ github.repository }}/${{ matrix.service }}:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.VPS_HOST }}
          username: ${{ secrets.VPS_USER }}
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            cd /opt/7spade
            docker stack deploy --with-registry-auth -c docker-compose.yml 7spade
```

For Option B, add `image:` directives to `docker-compose.yml` for each service pointing to the registry, and pin the deployed tag (e.g. via an env var the stack file interpolates). `docker stack deploy --with-registry-auth` forwards the manager's registry credentials to the nodes so they can pull private images; Swarm performs a rolling update of the changed services.

---

## Upgrading

To deploy a new version:

```bash
cd /opt/7spade
git pull origin main
docker compose build                                  # rebuild images
docker stack deploy -c docker-compose.yml 7spade      # rolling update of changed services
```

Database migrations are embedded in the API image and applied automatically on startup. No manual migration step required.

To force a single service to restart its tasks (e.g. after an env change, without changing the image):

```bash
docker service update --force 7spade_api
docker service update --force 7spade_ws
```

To tear the stack down and redeploy from scratch:

```bash
docker stack rm 7spade
# wait for tasks to drain (docker stack ps 7spade shows nothing), then:
docker compose build && docker stack deploy -c docker-compose.yml 7spade
```

> `docker stack rm` removes services and networks but **not** named volumes, so `postgres_data` survives a teardown.

---

## Scaling Notes

This setup is designed for a single-server deployment supporting a few hundred concurrent players. Key limits:

| Concern | Single-server capacity |
|---|---|
| Concurrent WS connections | ~1,000 (depends on goroutine memory) |
| PostgreSQL connections | ~100 (default `max_connections`) |
| Redis room snapshots | ~10,000 rooms (memory-bound) |

### When to scale beyond single server

- **Multiple WS instances**: Requires Redis pub/sub for cross-instance room state, or sticky sessions on the load balancer
- **Separate PostgreSQL**: Move to a managed database (e.g., Supabase, Neon, AWS RDS) when connection count or query load grows
- **CDN for frontend**: Serve the static `web/dist` from Cloudflare Pages or Vercel instead of nginx, keep API/WS on the VPS
- **Horizontal WS scaling**: Replace in-memory room map with Redis pub/sub; rooms can be hosted on any WS instance

---

## Troubleshooting

### WS service fails to start

The WS service **requires Redis** and fails fast at startup if Redis is unreachable. Under Swarm the task will crash and be rescheduled by the restart policy until Redis is up. Check:

```bash
docker service logs 7spade_ws
# Look for: "failed to connect to redis" or similar
docker stack ps 7spade_ws --no-trunc   # see restart/error history per task
```

Fix: ensure Redis is healthy, then force the WS tasks to restart.

```bash
docker service logs 7spade_redis
docker service update --force 7spade_ws
```

### JWT secret mismatch

If players can log in but get `401 Unauthorized` on WebSocket connections, the `JWT_SECRET` values differ between the `api` and `ws` services. Verify both `.env` files have the same value.

### CORS errors in browser

The API rejects credentialed browser requests unless the origin is in `CORS_ALLOWED_ORIGINS`. If you see:

```
Access to XMLHttpRequest ... blocked by CORS policy
```

Add the frontend origin (including the scheme) to `CORS_ALLOWED_ORIGINS` in `services/api/.env` and restart the API service.

### WebSocket fails silently behind nginx

If the WebSocket connects but immediately disconnects, verify the nginx config includes:

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

Without all three headers, nginx drops the upgrade and the WS handshake fails.

### Orphan rooms in lobby list

Rooms created via the API but never connected over WebSocket linger in the lobby list. The orphan-room reconcile job (every ~60s from the WS service) cleans these up after a 2-minute TTL. If rooms persist beyond a few minutes, check that `API_URL` and `INTERNAL_API_SECRET` are correctly set on the WS service.

### Build fails with Go version error

The Dockerfiles use `golang:1.26-alpine`. If your local Go version differs, builds still work inside Docker. If running locally without Docker, ensure Go 1.26+ is installed.

### Frontend shows old version after deploy

The web service container uses nginx to serve the built `dist/` folder. After a deploy, ensure the image was rebuilt and the service updated to it:

```bash
docker compose build web
docker stack deploy -c docker-compose.yml 7spade   # rolls the web service to the new image
```

Browsers may cache the old JS bundle. A hard refresh (`Ctrl+Shift+R`) or clearing cache resolves this.
