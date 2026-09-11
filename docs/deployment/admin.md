# Admin Deployment

The checked-in infrastructure supports the deployment described here; this is
not evidence of a live rollout. The proposed hostname is `admin.spade.my.id`.
Confirm ownership and DNS/TLS before provisioning, and select a release that
includes the infrastructure changes listed below.
Do not run release auto-updates until first provisioning is complete.

## Deployment Contract

| Component | Required deployment behavior |
|---|---|
| Images | Both `build-images.yml` and `deploy.yml` build `api`, `ws`, `web`, `admin-api`, and `admin-web` |
| Admin client build | Both workflows compile `VITE_ADMIN_API_URL=/admin-api` into `admin-web`; it is not runtime configuration |
| Admin API image | `services/admin-api/Dockerfile` includes the `/admin-api` server binary and the one-off `/bootstrap-admin` executable |
| Stack image selection | `${IMAGE_TAG:-latest}` selects the tag for all five application images; datastore image versions are separate |
| `admin-api` service | One replica, `env_file: [admin-api.env]`, internal port `8082`, no published port |
| `admin-web` service | One replica, `env_file: [admin-web.env]`, published `3001:80`; startup CSP uses `SKIN_ASSETS_ORIGIN` |
| Browser route | `https://admin.spade.my.id/admin-api/*` goes through admin-web nginx to `http://admin-api:8082/*` on the stack network |
| Release deploy | Preflight existence of all five services before any image update, then update `api`, `ws`, `web`, `admin-api`, `admin-web` in that order, waiting for convergence |

The admin API shares PostgreSQL with the player API, but not player credentials.
The player API applies the shared schema migrations before admin startup or
bootstrap. Swarm ignores `depends_on`; neither service creation order nor an
admin `/health` response proves that the schema is ready.

## Prerequisites

1. Review [Prerequisites](./prerequisites.md), [Swarm](./swarm.md), and
   [CI/CD](./ci-cd.md). Have manager access, Docker access on the task's node,
   registry pull access, and an immutable tag available for all five images.
2. Confirm `admin.spade.my.id`, create its A record, and configure any AAAA record
   only if IPv6 routing and firewall policy are ready. Obtain a matching TLS
   certificate and redirect HTTP to HTTPS.
3. Prepare `/opt/7spade/admin-api.env` and `/opt/7spade/admin-web.env` using the
   complete tables below. Keep them outside Git and restrict access to operators.
4. Generate an independent admin signing secret and a persistent MFA encryption
   key, for example using `openssl rand -hex 32` separately for each. Store the
   MFA key in an encrypted secret backup before enrolling an administrator.
5. Provide all five real operations URLs. The app validates their presence in
   production but does not provision observability tools or secure those tools
   for you. Verify the destinations and their access controls.
6. Back up the shared database using [Database Backups](./database-backups.md).
   Confirm restore access and retain the matching MFA key separately.
7. Configure provider firewall policy, then verify externally that direct
   application ports are inaccessible. Permit public application traffic only
   through 80/443; restrict SSH and cluster management access separately.

Swarm publishes `3001` on the node/routing mesh, not just loopback. nginx's
`127.0.0.1:3001` upstream does not change that. Block `3000`, `3001`, `8080`,
`8081`, and `8082` from the public network on every node, over IPv4 and IPv6 as
applicable. `8082` must also remain unpublished in the stack. Docker networking
can bypass simple host firewall rules, so use provider filtering and test from
outside the VPS rather than trusting a host firewall status display. Keep
PostgreSQL, Redis, and WS inspection endpoints private too.

## Runtime Environment

### `admin-api.env`

Path: `/opt/7spade/admin-api.env`. Required below means required for this
production deployment, even where the loader has a development default.

| Variable | Requirement / default | Value or purpose |
|---|---|---|
| `PORT` | Set explicitly; default `8082` | `8082`, matching admin-web's internal proxy |
| `DATABASE_URL` | Required | Shared player PostgreSQL database, e.g. `postgres://sevens:<password>@postgres:5432/sevens?sslmode=disable` on the private stack network; encode password URL characters |
| `APP_ENV` | Required; default `development` | Exactly `production`; enables production mutation MFA policy and required operations links |
| `ADMIN_JWT_SECRET` | Required | Independent high-entropy secret; never reuse player `JWT_SECRET` or other service secrets |
| `ADMIN_MFA_ENCRYPTION_KEY` | Required | Persistent high-entropy material used to encrypt TOTP secrets; retain across upgrades and restores |
| `ADMIN_FRONTEND_ORIGIN` | Required; default `http://localhost:5174` | Exactly `https://admin.spade.my.id`, without path, trailing slash, wildcard, or comma-separated alternatives |
| `ADMIN_SECURE_COOKIES` | Required; default `false` | Exactly `true`; do not weaken this to fix proxy problems |
| `API_HEALTH_URL` | Recommended; unset shows `not_configured` | `http://api:8080/health`, fetched by the admin server |
| `WS_HEALTH_URL` | Recommended; unset shows `not_configured` | `http://ws:8081/health`, fetched by the admin server; a VIP response does not verify every replica |
| `OPERATIONS_METRICS_URL` | Required in production | Absolute HTTP(S) URL to the metrics dashboard; use HTTPS for browser destinations |
| `OPERATIONS_LOGS_URL` | Required in production | Absolute HTTP(S) URL to access-controlled logs |
| `OPERATIONS_TRACES_URL` | Required in production | Absolute HTTP(S) URL to access-controlled traces |
| `OPERATIONS_DEPLOYMENTS_URL` | Required in production | Absolute HTTP(S) URL to deployment history / Actions |
| `OPERATIONS_RUNBOOK_URL` | Required in production | Absolute HTTP(S) URL to the operator runbook |
| `WS_ADMIN_URL` | Optional, paired with secret | `http://ws:8081`, base URL for redacted live-room inspection |
| `WS_ADMIN_SERVICE_SECRET` | Optional, paired with URL | Must equal `WS_INSPECTION_SECRET` in `ws.env` on every WS replica; server-only |
| `S3_ENDPOINT` | Optional storage group | S3-compatible API endpoint, not the public CDN URL |
| `S3_REGION` | Default `us-east-1` | Provider signing region; use `auto` for R2 if appropriate |
| `S3_BUCKET` | Optional storage group | Skin asset bucket, separate from database backups |
| `S3_ACCESS_KEY_ID` | Optional storage group | Least-privilege asset storage credential |
| `S3_SECRET_ACCESS_KEY` | Optional storage group | Paired secret; never send to browsers |
| `S3_PUBLIC_URL` | Optional; needed for remote asset URLs | Public HTTPS CDN/bucket prefix, e.g. `https://assets.spade.my.id` |
| `S3_USE_PATH_STYLE` | Default `false` | `true` only if required by the storage provider |
| `SKIN_ASSET_CORS_ALLOWED_ORIGINS` | Required by seed uploader | Comma-separated player and admin frontend origins allowed to read assets and use presigned uploads |

All configured health, inspection, and operations URLs must be absolute
HTTP(S) URLs. Do not embed credentials in dashboard links. `ADMIN_SECURE_COOKIES`
and the exact origin are operator requirements, not values automatically forced
by `APP_ENV=production`.

The following variables are accepted only for the one-off bootstrap procedure;
**do not put them in the persistent env file or Swarm service spec**:

| Variable | Requirement | Purpose |
|---|---|---|
| `ADMIN_BOOTSTRAP_EMAIL` | Required for bootstrap | First administrator's email |
| `ADMIN_BOOTSTRAP_PASSWORD` | Required for bootstrap | Unique strong password from a password manager; at most 72 characters (bcrypt truncates beyond 72 bytes) |
| `ADMIN_BOOTSTRAP_NAME` | Required for bootstrap | Display name |

### `admin-web.env`

Path: `/opt/7spade/admin-web.env`.

| Variable | Requirement / default | Value or purpose |
|---|---|---|
| `SKIN_ASSETS_ORIGIN` | Optional; empty when remote images are unused | One trusted HTTPS image origin, e.g. `https://assets.spade.my.id`; use the origin of `S3_PUBLIC_URL`, not storage credentials or a private S3 endpoint |

The image sets `NGINX_ENVSUBST_FILTER=^SKIN_ASSETS_ORIGIN$`; leave that restriction
intact. CSP is rendered at container startup. Do not put `VITE_ADMIN_API_URL` in
this env file: the client URL must already be compiled as `/admin-api` in both
image workflows. Changing a compiled URL requires rebuilding the image.

```bash
chmod 600 /opt/7spade/admin-api.env /opt/7spade/admin-web.env
```

`env_file` is not a Swarm secret store: values become part of the service spec
and are accessible to privileged Docker operators. Do not paste full service
inspection output or rendered stack configuration into tickets or CI logs.
After changing any env file, deploy the stack again with the intended
`IMAGE_TAG`; `docker service update --force` alone does not reread the files.

### Optional Integrations

For WS inspection, set both admin variables and the matching `WS_INSPECTION_SECRET`
on **all** WS replicas. Redeploy WS and admin configuration together. Leave the
pair unset if inspection is not needed; ordinary admin functions do not require
it. Use a dedicated secret, not `INTERNAL_API_SECRET`, and never expose it to
browser configuration or public probes.

For skin storage, configure the endpoint, bucket, region, credentials, public
URL, and trusted CORS origins. Without storage the admin can still
start and serve other functions. `S3_PUBLIC_URL` alone can resolve read-only
previews, but cannot upload or verify new objects for publishing. The full
storage group is needed for writes. Verify public asset reads without exposing
credentials, keep the asset bucket backed up, and allow only its trusted origin
in `SKIN_ASSETS_ORIGIN`. Do not broaden CSP to `*` to hide a URL mismatch.

## Proxy And TLS

The request path is:

```text
Browser HTTPS admin.spade.my.id
  -> outer nginx TLS -> 127.0.0.1:3001
  -> admin-web nginx /admin-api/ -> admin-api:8082/ (private stack network)
```

The outer admin virtual host needs `client_max_body_size 6m`, as does the inner
`/admin-api/` proxy, so the API's supported 5 MiB asset plus multipart framing
fits. Keep the outer host dedicated to admin-web; do not add a public upstream
or published port for admin-api. See [Reverse Proxy](./reverse-proxy.md).

The inner proxy strips `/admin-api/` via the trailing slash on `proxy_pass` and
must rewrite the refresh cookie path on both issuance and deletion:

```nginx
location /admin-api/ {
    client_max_body_size 6m;
    proxy_pass http://admin-api:8082/;
    proxy_cookie_path /auth /admin-api/auth;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The API emits the HttpOnly refresh cookie at `/auth`; the browser must receive
`Path=/admin-api/auth`. Its readable CSRF cookie stays at `/`. Keep both cookies
host-only, Secure, and SameSite=Strict. The inner HTTP hop does not disable Secure
cookies because `ADMIN_SECURE_COOKIES=true` explicitly controls issuance. A
successful password login alone does not prove the cookie rewrite works: verify
refresh and logout through the public HTTPS route.

## First Provisioning

1. Confirm the selected images contain the deployment contract above, including
   `/bootstrap-admin` and the cookie rewrite. Restrict the admin host to operator
   access until provisioning and verification are complete.
2. Prepare the stack, four runtime env files (`api.env`, `ws.env`,
   `admin-api.env`, `admin-web.env`), registry access, and deploy-shell
   `POSTGRES_PASSWORD`. Export an immutable `IMAGE_TAG` available for all five
   images; do not silently fall back to `latest` for this rollout.
3. Deploy/update the player API first to migrate the database. For a new stack,
   use an operator-reviewed initial stack copy with both admin services at zero
   replicas, start the datastores and player services, then wait for the player
   API. For an existing player stack, update `7spade_api` to the selected image
   first. Do not use the release workflow to create missing services.
4. Verify the new API task logs show `database: initialized successfully` with
   no migration failures. Check `/health` and inspect `schema_migrations` in the
   shared database against the migration filenames shipped in that exact API
   release. A healthy old API task or database ping is not sufficient.
5. Only after schema readiness, deploy the final stack with one replica each of
   `admin-api` and `admin-web`. Confirm both converge, then bootstrap using the
   procedure below. Install/reload the outer nginx configuration after `nginx -t`
   and verify the certificate and the end-to-end health route.
6. Enroll MFA, re-login, and complete the smoke checklist. Record the tag, image
   digests, migration verification, and operator approval without recording
   credentials. Only then enable access and release auto-updates.

Read-only checks for the operator, on the manager / relevant task node:

```bash
docker stack services 7spade
docker service ps 7spade_api --no-trunc
docker service logs --since 15m 7spade_api
docker service ps 7spade_admin-api --no-trunc
docker service logs --since 15m 7spade_admin-api
docker service logs --since 15m 7spade_admin-web
```

On the PostgreSQL task's node, select its running container and check the
migration ledger and bootstrap role (adjust database/user if configured otherwise):

```bash
docker ps --filter label=com.docker.swarm.service.name=7spade_postgres
docker exec -i <postgres-container-id> psql -U sevens -d sevens -v ON_ERROR_STOP=1 \
  -c 'SELECT version, applied_at FROM schema_migrations ORDER BY version;'
docker exec -i <postgres-container-id> psql -U sevens -d sevens -v ON_ERROR_STOP=1 \
  -c "SELECT name FROM admin_roles WHERE name = 'super_admin';"
```

Require the expected migration set and a `super_admin` row; do not create roles
or mark migrations applied by hand to bypass a failed migration.

```bash
curl --fail --silent --show-error https://api.spade.my.id/health
curl --fail --silent --show-error https://admin.spade.my.id/admin-api/health
```

Admin health returns `{"status":"ok","service":"admin-api"}`. It is a
**liveness response only**, not a schema or ongoing database readiness check.
After bootstrap, authenticated `/me` and `/dashboard` reads must also succeed.

## One-Off Bootstrap

Bootstrap creates the first administrator with the `super_admin` role and
refuses if **any administrator already exists**. It is not password recovery
or a way to add subsequent admins. Do not delete existing administrators to make
it run again; use the invitation flow for additional operators. Run only one
bootstrap process, after the migration/role checks above.

Locate the task on the manager, then open a trusted interactive Bash shell on
the node running it. Docker exec addresses a local container, not a Swarm service:

```bash
docker service ps --filter desired-state=running 7spade_admin-api
# On the task's node:
docker ps --filter label=com.docker.swarm.service.name=7spade_admin-api
```

Run this in that **Bash** shell with existing Docker permission. Enter the
selected container ID and credentials at the prompts, not as command arguments.
The subshell, cleanup trap, and hidden password input keep the temporary values
out of the parent environment and ordinary shell history. Do not use a recorded
terminal, shell tracing, CI job, or an untrusted Docker host for bootstrap.

```bash
(
  set +x
  set -e
  trap 'unset ADMIN_BOOTSTRAP_EMAIL ADMIN_BOOTSTRAP_PASSWORD ADMIN_BOOTSTRAP_NAME admin_cid' EXIT
  read -r -p 'Running admin-api container ID: ' admin_cid
  test -n "$admin_cid"
  test "$(docker inspect --format '{{index .Config.Labels "com.docker.swarm.service.name"}}' "$admin_cid")" = '7spade_admin-api'
  read -r -p 'First administrator email: ' ADMIN_BOOTSTRAP_EMAIL
  read -r -p 'First administrator display name: ' ADMIN_BOOTSTRAP_NAME
  read -r -s -p 'First administrator password: ' ADMIN_BOOTSTRAP_PASSWORD
  printf '\n'
  test -n "$ADMIN_BOOTSTRAP_EMAIL"
  test -n "$ADMIN_BOOTSTRAP_NAME"
  test -n "$ADMIN_BOOTSTRAP_PASSWORD"
  export ADMIN_BOOTSTRAP_EMAIL ADMIN_BOOTSTRAP_PASSWORD ADMIN_BOOTSTRAP_NAME
  docker exec \
    --env ADMIN_BOOTSTRAP_EMAIL \
    --env ADMIN_BOOTSTRAP_PASSWORD \
    --env ADMIN_BOOTSTRAP_NAME \
    "$admin_cid" /bootstrap-admin
)
```

Docker forwards the exported values by name to that exec process, which inherits
the container's existing `DATABASE_URL` and `APP_ENV`. No password is expanded
into the command line and no bootstrap secret is added to the long-running
service spec. Privileged host/Docker access can still observe process environment;
this is not protection from a compromised operator machine or daemon. Avoid a
`sudo` boundary that strips the exported variables; use an already authorized
operator shell rather than putting secret values into `sudo` arguments.

The trap cleans up on success or failure, and the subshell exits. If credentials
were separately exported in another shell, unset them there too. Confirm no
`ADMIN_BOOTSTRAP_*` entries remain in env files, deployment automation, or the
service spec. If any were persisted, remove them, redeploy with the correct tag,
and treat the password as exposed. Retain the account password in the approved
password manager, not in deployment files or logs.

Sign in over HTTPS, open Security, enroll an authenticator, confirm a TOTP code,
and store the one-time recovery codes securely. Then **log out and log in again**,
completing the MFA challenge. Enrollment does not upgrade the original session
to MFA-verified, so that old session remains unable to perform production
mutations. Confirm server and authenticator clocks are synchronized.

## Smoke Checklist

The Tests workflow runs an isolated container regression check. To run it
locally without touching existing services:

```bash
docker build -t 7spade-admin-api:deploy-test services/admin-api
docker build -t 7spade-admin-web:deploy-test admin-web
node scripts/admin-deployment-smoke.mjs
```

This requires Node 22+ and Docker. It creates disposable PostgreSQL and admin
containers, applies the shared migrations, tests bootstrap, cookie paths,
refresh/logout, MFA, CSP, and the inner proxy upload limit, then cleans up. It
uses loopback HTTP with a path-aware cookie jar, not a browser TLS test, and
does not configure real asset storage. Complete the HTTPS and external-network
checks below on the deployed environment as well.

Use controlled test accounts and draft resources for write tests. Do not suspend
real users, publish rewards, or change live feature settings just to test access.
Keep tokens, passwords, recovery codes, cookies, and investigation payloads out
of screenshots, saved HAR files, and tickets.

- [ ] All five application services exist; API and WS are `3/3`, web and both
  admin services `1/1`. Confirm running images/digests match the intended release.
- [ ] Migration ledger matches the API release; admin role seed exists. Public
  player and admin health routes succeed; authenticated `/me` and `/dashboard`
  work and dashboard health/link destinations are correct.
- [ ] Admin hostname resolves correctly, TLS is valid, HTTP redirects to HTTPS,
  and direct ports are inaccessible from an external IPv4/IPv6 client on every
  node. No admin-api port is published.
- [ ] Browser requests use same-origin `/admin-api`, never localhost or a player
  API host. Check CSP and other security response headers on the admin page.
- [ ] Login issues a host-only Secure, HttpOnly, SameSite=Strict refresh cookie
  with `Path=/admin-api/auth`; the Secure CSRF cookie remains readable at `/`.
  A page reload/refresh request rotates the session successfully through the
  public proxy with the frontend's `X-CSRF-Token` header.
- [ ] Logout clears the refresh cookie at the same rewritten path and revokes
  the session. Reload does not restore it; old access/refresh credentials fail.
- [ ] Before MFA enrollment, an authenticated permitted GET still succeeds.
  A controlled bearer-authenticated mutation fails `403 MFA required`.
  Enrollment and confirmation remain accessible; after confirmation the old
  session is still blocked until logout and MFA re-login.
- [ ] After enrollment, password login returns an MFA challenge, invalid TOTP
  fails, and a valid TOTP completes login. Test a recovery code on a controlled
  account and verify it cannot be reused. Protect the remaining codes.
- [ ] A permitted mutation succeeds only after MFA re-login and records an audit
  event. A lower-privilege test administrator is denied a mutation without the
  required permission even with MFA. Unauthenticated and player-token requests
  to protected admin endpoints fail `401`.
- [ ] Missing/incorrect CSRF on refresh/logout is rejected. Other authenticated
  mutations use bearer authentication, MFA, and permissions; do not claim a
  blanket CSRF-header gate on them. A disallowed browser Origin is denied; do
  not equate CORS with authentication or network isolation.
- [ ] If configured, redacted WS inspection works without leaking the service
  credential. If storage is configured, test a draft skin upload and preview,
  including a supported near-5-MiB file through both proxies, without CSP errors.
  If optional integrations are absent, their unavailability is expected and
  ordinary admin functions still work.

Production MFA is **not a blanket read gate**. `RequireAuth` gates methods other
than GET/HEAD/OPTIONS, except MFA enroll/confirm. Public authentication endpoints
have their own checks. Reads still require authentication and route permissions,
and a POST investigation endpoint can be MFA-gated despite being read-like in
purpose. Do not report permitted pre-enrollment reads as a failed MFA deployment.

## Upgrades And Rollback

Before release auto-updates, manually provision both new services, env files,
network policy, proxy/TLS, schema, and first admin. The workflow only updates
images; it cannot create services, install nginx, or reread server env files.
Its all-five existence preflight must fail before **any** update if one is
missing. That check is not schema readiness or a transactional five-service
rollout: later failures can leave mixed versions.

Back up PostgreSQL and the MFA key, review migrations for compatibility with old
and new services, and retain previous image digests before upgrading. The release
workflow updates `api` first, then `ws`, `web`, `admin-api`, `admin-web`, waiting
for each update to converge. Verify API migration completion and run the smoke
checks; convergence is not a substitute for application readiness.

For a manual rollout, apply the same API-first schema gate before deploying the
final stack. A single `docker stack deploy` does not order migrations for you.
Use `IMAGE_TAG=<immutable-tag>` in the deploy shell for all five images, defaulting
to `latest` only deliberately. Record that tag in the operator deployment
configuration: the image-update workflow does not persist `IMAGE_TAG` in your
shell or server stack file. A later env-only stack deploy must use the same tag
or it may unintentionally change every application image.

Rollback only to images compatible with the current shared schema. A service
rollback or older `IMAGE_TAG` does **not** undo migrations; do not automatically
restore an old database just to roll back admin. Schema recovery affects players,
sessions, audit history, and all other shared data, can lose post-backup writes,
and requires a coordinated maintenance/restore plan. Prefer a compatible forward
fix when possible. After a restore, use the MFA encryption key that matches the
restored enrollments and recheck session revocation and MFA login.

Keep `ADMIN_MFA_ENCRYPTION_KEY` unchanged during routine upgrades. Rotating the
JWT signing secret invalidates existing access tokens but is not a substitute
for revoking stored refresh sessions. Back up secret configuration and asset
bytes separately from PostgreSQL; test recovery in an isolated environment, not
by experimenting against production.

## Troubleshooting

| Symptom | Check / action |
|---|---|
| Release preflight reports a missing service | Stop; complete manual first provisioning for all five services before retrying. Do not bypass preflight and partially update the player stack |
| Admin task exits at startup | Check `DATABASE_URL`, required signing/MFA keys, and all five operations URLs. Use `APP_ENV=production`, not development as a workaround |
| Health is OK but login/dashboard fails | Admin health is liveness-only. Verify player API migration completion, the exact release's migration ledger, shared DB selection, and seeded roles |
| `/bootstrap-admin` not found | The selected image lacks the agreed bootstrap binary. Build/deploy the correct image; do not install Go or copy secrets into the running container |
| Bootstrap refuses an existing administrator | Expected protection, including when an existing account is disabled. Use normal access/invitations or an approved recovery procedure, not deletion/rebootstrap |
| Login works but refresh fails or logout leaves a cookie | Check `proxy_cookie_path /auth /admin-api/auth`, trailing-slash proxy routing, Secure cookies over HTTPS, and CSRF header/cookie. Clear stale cookies from old proxy paths and retest |
| Browser CORS failure | Match `ADMIN_FRONTEND_ORIGIN` exactly to the HTTPS admin origin and rebuild if `VITE_ADMIN_API_URL` was wrong; do not use wildcard credentialed CORS |
| `403 MFA required` after enrollment | Log out and complete a fresh MFA login; confirmation does not elevate the current session. Check permissions separately if a mutation still fails |
| TOTP fails after restore/redeploy | Check clock synchronization and restore the original MFA encryption key; generating a new key cannot decrypt old enrollments |
| Dashboard health is `not_configured` / `unreachable` | Set the health URLs or check internal service DNS/network access. A VIP health probe alone does not check every WS replica |
| WS inspection unavailable | Confirm optional URL/secret pairing and matching secret on every WS replica; never debug by publishing the internal endpoint |
| Upload returns `413` | Both the outer admin virtual host and inner API location need `client_max_body_size 6m`; the API still enforces its own 5 MiB asset limit |
| Skin upload/preview fails | Check complete S3 write config, public URL readability, and `SKIN_ASSETS_ORIGIN` CSP. A public-URL-only signer cannot write objects |
| Env edit appears ignored | Redeploy the stack with the recorded `IMAGE_TAG`; a forced restart alone retains the old service environment |
| Direct port `3001` is reachable publicly | Close it at the provider firewall on all nodes/address families; loopback nginx upstreams do not constrain Swarm publishing |

See also [Operations](./operations.md), [Troubleshooting](./troubleshooting.md),
and the [Admin API security model](../admin-api.md).
