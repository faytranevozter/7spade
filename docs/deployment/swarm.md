# Swarm Deployment

The server does not build images. The agreed image workflows publish `api`,
`ws`, `web`, `admin-api`, and `admin-web` to GitHub Container Registry. The VPS
runs [`deployment/stack.yml`](../../deployment/stack.yml). Verify the
[admin deployment contract](./admin.md#deployment-contract) is present in the
selected release before following this five-service procedure.

## 1. Prepare the Deploy Directory

```bash
sudo mkdir -p /opt/7spade
```

Copy the current stack file to the server as `/opt/7spade/stack.yml`:

```bash
scp deployment/stack.yml <user>@<vps>:/tmp/stack.yml
ssh <user>@<vps> 'sudo mv /tmp/stack.yml /opt/7spade/stack.yml'
```

If you are already on the server and have the repo checked out:

```bash
sudo cp deployment/stack.yml /opt/7spade/stack.yml
```

The agreed stack references
`ghcr.io/faytranevozter/7spade/{api,ws,web,admin-api,admin-web}:${IMAGE_TAG:-latest}`
and retains 3 API and 3 WS replicas. Export an immutable `IMAGE_TAG` available
for all five images, such as a release tag published by `deploy.yml`, for
reproducible deployments. The default `latest` applies to all five images.

## 2. Create Runtime Env Files

Create:

```text
/opt/7spade/api.env
/opt/7spade/ws.env
/opt/7spade/admin-api.env
/opt/7spade/admin-web.env
```

Use [Environment](./environment.md) for required values. Export
`POSTGRES_PASSWORD` in the deploy shell and make it match the password embedded
in `DATABASE_URL`:

```bash
export POSTGRES_PASSWORD='<strong database password>'
```

The current stack interpolates this environment variable directly; using a
Swarm secret would first require changing `deployment/stack.yml` to consume a
secret file.

For the current 3-replica WS stack, `ws.env` should include:

```env
WS_REDIS_URL=redis://redis-ws:6379
```

## 3. Authenticate to GHCR

If the packages are private, log in on the server with a token that has `read:packages`:

```bash
echo "<GHCR_READ_TOKEN>" | docker login ghcr.io -u <github-user> --password-stdin
```

If the packages are public, skip this step.

## 4. Deploy

First provisioning is manual: follow [Admin Deployment](./admin.md#first-provisioning)
to migrate through the player API before starting admin tasks or bootstrap.
For a new stack, hold both admin services at zero replicas in an initial
operator-reviewed stack copy, verify schema readiness, then deploy the final
stack. For an existing player stack, update and verify the API first. A single
stack deploy does not enforce that order. Do not invoke release auto-updates
until all five services have been provisioned and verified.

After the API-first readiness gate, deploy the final stack:

```bash
cd /opt/7spade
export IMAGE_TAG=vX.Y.Z # replace with a tag published for all five images
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

Swarm ignores `depends_on`. Check API migration logs and the database migration
ledger for the selected release, then admin authenticated reads as well as
health. Admin `/health` is liveness-only. Service convergence alone is not schema
readiness.

Verify services:

```bash
docker stack services 7spade
```

Expected shape after first provisioning:

| Service | Expected replicas |
|---|---:|
| `7spade_postgres` | `1/1` |
| `7spade_redis` | `1/1` |
| `7spade_redis-ws` | `1/1` |
| `7spade_api` | `3/3` |
| `7spade_ws` | `3/3` |
| `7spade_web` | `1/1` |
| `7spade_admin-api` | `1/1` |
| `7spade_admin-web` | `1/1` |

`admin-api` uses `admin-api.env` with no published port. `admin-web` uses
`admin-web.env` and publishes `3001:80`. This is **not loopback-only**; provider
firewall policy and external reachability checks must prevent bypassing TLS on
direct ports. See [Reverse Proxy](./reverse-proxy.md).

Inspect tasks if replicas do not converge:

```bash
docker stack ps 7spade --no-trunc
```

## 5. Install nginx

```bash
sudo apt update
sudo apt install -y nginx
sudo systemctl enable nginx
```

Continue with [Reverse Proxy](./reverse-proxy.md).
