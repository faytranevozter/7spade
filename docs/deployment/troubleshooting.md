# Troubleshooting

## WS Service Fails To Start

The WS service requires Redis and fails fast if Redis is unreachable. Under Swarm, the task crashes and is rescheduled until dependencies are available.

Check logs and task history:

```bash
docker service logs 7spade_ws
docker stack ps 7spade_ws --no-trunc
```

Fix Redis first, then force WS tasks to restart if needed:

```bash
docker service logs 7spade_redis
docker service logs 7spade_redis-ws
docker service update --force 7spade_ws
```

## JWT Secret Mismatch

If players can log in but get `401 Unauthorized` on WebSocket connections, `JWT_SECRET` differs between `api.env` and `ws.env`. Both services must use the same value.

## CORS Errors In Browser

The API rejects credentialed browser requests unless the origin is listed in `CORS_ALLOWED_ORIGINS`.

Add the frontend origin, including scheme, to `/opt/7spade/api.env`, then redeploy:

```bash
cd /opt/7spade
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

Do not rely on `docker service update --force` for env-file changes; Swarm reads `env_file` values at deploy time.

## WebSocket Fails Behind nginx

Verify [`deployment/nginx/7spade.conf`](../../deployment/nginx/7spade.conf) is installed and includes:

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

Without these headers, nginx drops the upgrade and the WS handshake fails.

## Orphan Rooms In Lobby List

Rooms created through the API but never connected over WebSocket linger until the WS reconcile job cleans them up. If rooms persist beyond a few minutes, check that `API_URL` and `INTERNAL_API_SECRET` are correctly set in `ws.env`.

## Image Build Fails In CI

Images build in [Build images](../../.github/workflows/build-images.yml), not on the server. If a build fails, check the workflow logs. If the push step receives `403`, confirm GitHub Actions workflow permissions are set to read/write.

## Frontend Shows Old Version After Deploy

The web service serves the static bundle baked into the image. Confirm the running image:

```bash
docker service inspect 7spade_web --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}'
```

Force a re-pull of the referenced tag:

```bash
docker service update --force --with-registry-auth 7spade_web
```

If URLs point at localhost, the `VITE_*` repository variables were wrong when CI built the image. Fix the variables, rerun [Build images](../../.github/workflows/build-images.yml), then redeploy.

Browsers can also cache old JS bundles. Use a hard refresh or clear cache.
