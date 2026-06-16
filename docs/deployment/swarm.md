# Swarm Deployment

The server does not build images. GitHub Actions builds and publishes `api`, `ws`, and `web` images to GitHub Container Registry. The VPS only runs `docker stack deploy` against [`deployment/stack.yml`](../../deployment/stack.yml).

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

The current stack references `ghcr.io/faytranevozter/7spade/{api,ws,web}:latest` and runs 3 API replicas plus 3 WS replicas. Pin to an immutable tag, such as a git SHA or `vX.Y.Z`, if you need reproducible rollbacks.

## 2. Create Runtime Env Files

Create:

```text
/opt/7spade/api.env
/opt/7spade/ws.env
```

Use [Environment](./environment.md) for required values. Set `POSTGRES_PASSWORD` in the deploy shell or via a Swarm secret, and make it match the password embedded in `DATABASE_URL`.

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

```bash
cd /opt/7spade
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

Swarm ignores `depends_on` in the stack file. Services converge through health checks, restart policies, and application startup behavior.

Verify services:

```bash
docker stack services 7spade
```

Expected current production shape:

| Service | Expected replicas |
|---|---:|
| `7spade_postgres` | `1/1` |
| `7spade_redis` | `1/1` |
| `7spade_redis-ws` | `1/1` |
| `7spade_api` | `3/3` |
| `7spade_ws` | `3/3` |
| `7spade_web` | `1/1` |

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
