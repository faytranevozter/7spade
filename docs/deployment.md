# Deployment Guide

This guide covers deploying Seven Spade to production on a single VPS using Docker Compose behind an nginx reverse proxy with TLS.

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
| Docker | 24+ with Docker Compose v2 |
| Domain | One domain with three A records pointing to the VPS |
| Ports | 80, 443 open to the internet |

Install Docker and Compose on a fresh Ubuntu/Debian server:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# log out and back in to apply group change
docker compose version   # verify
```

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

> **Security:** Generate a strong `JWT_SECRET` with `openssl rand -base64 32`. The API and WS services must share the same value. `INTERNAL_API_SECRET` must also match across both services.

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

### 5. Update docker-compose.yml for production

The default `docker-compose.yml` uses build context with no image tag. For production you may want to pin images, but the default works as-is for a single-server deploy.

No changes needed to `docker-compose.yml` if you are deploying the whole stack on one server.

### 6. Build and start the stack

```bash
docker compose up -d --build
```

Verify all services are running:

```bash
docker compose ps
```

Expected output: 5 services, all healthy.

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

Also check Docker health:

```bash
docker compose ps
# All services should show "Up" and "(healthy)" for postgres/redis
```

Add these as uptime monitor targets (e.g., UptimeRobot, Better Stack, or a simple cron):

```bash
# Cron: check every 5 minutes, alert if down
*/5 * * * * curl -sf https://api-spade.example.com/health >/dev/null || echo "API DOWN" | mail -s "7spade alert" ops@example.com
```

---

## Backups

### PostgreSQL

Schedule a daily `pg_dump` from inside the container:

```bash
# Add to crontab: backup every day at 3am UTC
0 3 * * * docker compose exec -T postgres pg_dump -U sevens sevens | gzip > /opt/backups/sevens-$(date +\%Y\%m\%d).sql.gz
```

Rotate old backups (keep last 30 days):

```bash
find /opt/backups -name "sevens-*.sql.gz" -mtime +30 -delete
```

Restore:

```bash
gunzip -c /opt/backups/sevens-20250101.sql.gz | docker compose exec -T postgres psql -U sevens sevens
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
# All services
docker compose logs -f

# Single service
docker compose logs -f api
docker compose logs -f ws
```

### Resource usage

```bash
docker stats --no-stream
```

### Database connections

```bash
docker compose exec postgres psql -U sevens -c "SELECT count(*) FROM pg_stat_activity WHERE datname='sevens';"
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
            docker compose up -d --build
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
            docker compose pull
            docker compose up -d
```

For Option B, add `image:` directives to `docker-compose.yml` for each service pointing to the registry.

---

## Upgrading

To deploy a new version:

```bash
cd /opt/7spade
git pull origin main
docker compose up -d --build
```

Database migrations are embedded in the API image and applied automatically on startup. No manual migration step required.

To restart without rebuilding:

```bash
docker compose restart api ws
```

To fully recreate:

```bash
docker compose down && docker compose up -d --build
```

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

The WS service **requires Redis** and fails fast at startup if Redis is unreachable. Check:

```bash
docker compose logs ws
# Look for: "failed to connect to redis" or similar
```

Fix: ensure Redis is healthy before WS starts.

```bash
docker compose logs redis
docker compose restart ws
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

The web service container uses nginx to serve the built `dist/` folder. After a deploy, ensure the container was rebuilt:

```bash
docker compose up -d --build web
```

Browsers may cache the old JS bundle. A hard refresh (`Ctrl+Shift+R`) or clearing cache resolves this.
