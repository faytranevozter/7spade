# Environment

Runtime config for `api` and `ws` lives in env files on the server and is referenced by [`deployment/stack.yml`](../../deployment/stack.yml). Build-time config for `web` is baked into the static client bundle and is not set on the server.

## `api.env`

Example path on the VPS: `/opt/7spade/api.env`.

| Variable | Required | Example |
|---|---|---|
| `PORT` | Yes | `8080` |
| `DATABASE_URL` | Yes | `postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable` |
| `REDIS_URL` | Yes | `redis://redis:6379` |
| `JWT_SECRET` | Yes | `<32+ char random string>` |
| `OAUTH_STATE_SECRET` | No | Falls back to `JWT_SECRET` if empty |
| `INTERNAL_API_SECRET` | Yes | `<shared secret matching ws service>` |
| `FRONTEND_URL` | Yes | `https://spade.example.com` |
| `CORS_ALLOWED_ORIGINS` | Yes | `https://spade.example.com,https://api-spade.example.com` |
| `LEADERBOARD_MIN_GAMES` | No | Default `5` |
| `RATE_LIMIT_AUTH_PER_MINUTE` | No | Default `10` (per IP) |
| `RATE_LIMIT_ROOMS_WRITE_PER_MINUTE` | No | Default `5` (per user) |
| `RATE_LIMIT_SOCIAL_PER_MINUTE` | No | Default `30` (per user) |
| `RATE_LIMIT_GENERAL_PER_MINUTE` | No | Default `60` (per identity) |
| `RATE_LIMIT_WINDOW_SECONDS` | No | Default `60` |
| `RATE_LIMIT_QUICK_PLAY_COOLDOWN_MS` | No | Default `3000` |
| `SMTP_HOST` | No | SMTP host; when unset, email links are logged |
| `SMTP_PORT` | No | Default `587` |
| `SMTP_USER` | No | SMTP username |
| `SMTP_PASS` | No | SMTP password |
| `SMTP_FROM` | No | Default `no-reply@sevenspade.local` |
| `SMTP_FROM_NAME` | No | Default `Seven Spade` |
| `SMTP_REPLY_TO` | No | Optional Reply-To address |
| `SMTP_ENCRYPTION` | No | `auto`, `tls`, `starttls`, or `none` |
| `GOOGLE_OAUTH_CLIENT_ID` | Optional | Google OAuth client ID |
| `GOOGLE_OAUTH_CLIENT_SECRET` | Optional | Google OAuth client secret |
| `GOOGLE_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/google` |
| `GITHUB_OAUTH_CLIENT_ID` | Optional | GitHub OAuth client ID |
| `GITHUB_OAUTH_CLIENT_SECRET` | Optional | GitHub OAuth client secret |
| `GITHUB_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/github` |
| `TELEGRAM_OAUTH_CLIENT_ID` | Optional | Telegram OIDC client ID |
| `TELEGRAM_OAUTH_CLIENT_SECRET` | Optional | Telegram OIDC client secret |
| `TELEGRAM_OAUTH_REDIRECT_URL` | Optional | `https://spade.example.com/auth/callback/telegram` |

Minimal example:

```env
PORT=8080
DATABASE_URL=postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=<32+ char random string>
INTERNAL_API_SECRET=<shared secret matching ws>
FRONTEND_URL=https://spade.example.com
CORS_ALLOWED_ORIGINS=https://spade.example.com,https://api-spade.example.com
```

## `ws.env`

Example path on the VPS: `/opt/7spade/ws.env`.

| Variable | Required | Example |
|---|---|---|
| `PORT` | Yes | `8081` |
| `DATABASE_URL` | Yes | `postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable` |
| `REDIS_URL` | Yes | `redis://redis:6379` |
| `WS_REDIS_URL` | Yes for the current 3-replica production stack | `redis://redis-ws:6379` |
| `JWT_SECRET` | Yes | Must match `api.env` |
| `API_URL` | Yes | `http://api:8080` |
| `INTERNAL_API_SECRET` | Yes | Must match `api.env` |

The current [`deployment/stack.yml`](../../deployment/stack.yml) runs 3 `ws` replicas, so `WS_REDIS_URL=redis://redis-ws:6379` should be set. Single-replica deployments may omit `redis-ws` and `WS_REDIS_URL`; the service falls back to `REDIS_URL`.

Minimal example for the current multi-replica stack:

```env
PORT=8081
DATABASE_URL=postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable
REDIS_URL=redis://redis:6379
WS_REDIS_URL=redis://redis-ws:6379
JWT_SECRET=<must match api JWT_SECRET>
API_URL=http://api:8080
INTERNAL_API_SECRET=<must match api INTERNAL_API_SECRET>
```

Lock down both files because they hold secrets:

```bash
chmod 600 /opt/7spade/api.env /opt/7spade/ws.env
```

Generate a strong JWT secret:

```bash
openssl rand -base64 32
```

## Build-Time Client Variables

The web image is built by [Build images](../../.github/workflows/build-images.yml), and these values are baked into the static bundle from GitHub Actions repository variables:

| Variable | Required | Example |
|---|---|---|
| `VITE_API_URL` | Yes | `https://api-spade.example.com` |
| `VITE_WS_URL` | Yes | `wss://wsspade.example.com` |
| `VITE_WS_HEALTH_URL` | Yes | `https://wsspade.example.com` |

## Current Production Values

Production uses the `fahrur.my.id` hostnames:

```env
FRONTEND_URL=https://spade.fahrur.my.id
CORS_ALLOWED_ORIGINS=https://spade.fahrur.my.id,https://api-spade.fahrur.my.id
GOOGLE_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/google
GITHUB_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/github
TELEGRAM_OAUTH_REDIRECT_URL=https://spade.fahrur.my.id/auth/callback/telegram
```

Web repository variables:

```env
VITE_API_URL=https://api-spade.fahrur.my.id
VITE_WS_URL=wss://wsspade.fahrur.my.id
VITE_WS_HEALTH_URL=https://wsspade.fahrur.my.id
```

Store real secret values outside the repo. The placeholders above intentionally omit passwords, JWT secrets, OAuth client secrets, and internal API secrets.
