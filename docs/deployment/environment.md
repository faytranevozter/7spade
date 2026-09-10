# Environment

Runtime config lives in server env files referenced by the agreed
[`deployment/stack.yml`](../../deployment/stack.yml): `api.env`, `ws.env`,
`admin-api.env`, and `admin-web.env`. Client API URLs are baked into the static
bundles, not configured at runtime. See [Admin Deployment](./admin.md) for the
required infrastructure changes and first-provisioning gate.

## `api.env`

Example path on the VPS: `/opt/7spade/api.env`.

| Variable | Required | Example |
|---|---|---|
| `PORT` | Yes | `8080` |
| `DATABASE_URL` | Yes | `postgres://sevens:<STRONG_PASSWORD>@postgres:5432/sevens?sslmode=disable` |
| `REDIS_URL` | Yes | `redis://redis:6379` |
| `JWT_SECRET` | Yes | `<32+ char random string>` |
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
| `TELEGRAM_MOBILE_REDIRECT_URL` | Optional | `https://api.spade.example.com/auth/mobile/telegram/callback` |
| `S3_ENDPOINT` | Optional | S3-compatible endpoint for skin assets |
| `S3_BUCKET` | Optional | Application asset bucket; keep separate from backups |
| `S3_REGION` | Optional | `auto` for R2 or the provider's signing region |
| `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | Optional | Least-privilege skin-asset upload credentials |
| `S3_PUBLIC_URL` | Optional | Public CDN or bucket URL prefix |
| `S3_USE_PATH_STYLE` | Optional | Provider-specific path-style addressing |

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

When the S3 variables are absent, the API starts in degraded mode: player
metadata remains available, but asset upload/publishing workflows cannot store
new bytes. The embedded seed uploader is `go run ./cmd/skinassets` from
`services/api`; run it with the production API and storage environment after
initial bucket/CDN setup. Verify uploaded objects are publicly readable without
credentials. Application assets and PostgreSQL backups should use separate
buckets or tightly separated prefixes and credentials.

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
| `WS_INSPECTION_SECRET` | Only when admin live-room inspection is deployed | Must match admin API `WS_ADMIN_SERVICE_SECRET`; never expose to browsers |

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

Lock down all runtime files; API files contain secrets:

```bash
chmod 600 /opt/7spade/api.env /opt/7spade/ws.env \
  /opt/7spade/admin-api.env /opt/7spade/admin-web.env
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
| `VITE_SKIN_ASSETS_URL` | Required to render remote skins | Public CDN or bucket prefix matching `S3_PUBLIC_URL` |

Both checked-in GitHub image workflows pass `VITE_SKIN_ASSETS_URL` to the web
Docker build. Set it before building; changing it requires a new web image.

Local Compose forwards the same `S3_*` variables to `api` and `admin-api`.
It also uses `S3_PUBLIC_URL` as the admin web's single additional CSP image
source at container startup. Set `S3_PUBLIC_URL` to the public HTTPS asset URL
(an origin or a URL prefix), not the private S3 API endpoint. The admin nginx
proxy permits a 6 MiB request so the API's supported 5 MiB file plus multipart
framing is not rejected at the edge.

## Admin Runtime And Build Configuration

The complete [admin-api.env table](./admin.md#admin-apienv) and
[admin-web.env table](./admin.md#admin-webenv) live in the admin runbook. Use
`APP_ENV=production`, `ADMIN_SECURE_COOKIES=true`, the exact
`ADMIN_FRONTEND_ORIGIN=https://admin.spade.my.id` (proposed hostname), an
independent `ADMIN_JWT_SECRET`, and a persistent, backed-up
`ADMIN_MFA_ENCRYPTION_KEY`. All five `OPERATIONS_*_URL` values are required in
production: metrics, logs, traces, deployments, and runbook.

The agreed image workflows both compile `VITE_ADMIN_API_URL=/admin-api` into
`admin-web`; no runtime env entry can change it. In Swarm, set the trusted public
asset **origin** explicitly as `SKIN_ASSETS_ORIGIN` in `admin-web.env` for startup
CSP rendering. Unlike local Compose, do not assume `S3_PUBLIC_URL` is forwarded
automatically. Both outer and inner admin proxies need the 6 MiB upload allowance.

Never persist `ADMIN_BOOTSTRAP_*` values in these files. Forward them only to the
one-off bootstrap process using the [safe procedure](./admin.md#one-off-bootstrap).
Optional WS inspection uses a dedicated shared secret on every WS replica;
optional S3 storage and CSP configuration are documented in the same runbook.

## Deploy-Shell Variables

| Variable | Purpose |
|---|---|
| `POSTGRES_PASSWORD` | Existing stack interpolation; must match database credentials |
| `IMAGE_TAG` | Agreed stack uses `${IMAGE_TAG:-latest}` for all five application images; export an available immutable tag for reproducibility |

These are stack interpolation inputs, not entries in application env files.
Do not rely on Compose's automatic `.env` loading for `docker stack deploy`.
Record the intended tag and export it again for env-only redeploys; release
service updates do not persist this shell setting. Protect env files and avoid
logging rendered stack/service specs, which can contain their secrets.

## Current Production Values

Production uses the `my.id` hostnames:

```env
FRONTEND_URL=https://spade.my.id
CORS_ALLOWED_ORIGINS=https://spade.my.id,https://api.spade.my.id
GOOGLE_OAUTH_REDIRECT_URL=https://spade.my.id/auth/callback/google
GITHUB_OAUTH_REDIRECT_URL=https://spade.my.id/auth/callback/github
TELEGRAM_OAUTH_REDIRECT_URL=https://spade.my.id/auth/callback/telegram
```

Web repository variables:

```env
VITE_API_URL=https://api.spade.my.id
VITE_WS_URL=wss://ws.spade.my.id
VITE_WS_HEALTH_URL=https://ws.spade.my.id
VITE_SKIN_ASSETS_URL=https://assets.spade.my.id
```

Store real secret values outside the repo. The placeholders above intentionally omit passwords, JWT secrets, OAuth client secrets, and internal API secrets.
