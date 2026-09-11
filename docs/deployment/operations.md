# Operations

## Health Checks

The player API and WebSocket service expose `/health` endpoints. After
[admin first provisioning](./admin.md#first-provisioning), also check its
same-origin route:

```bash
curl -s https://api.spade.my.id/health | jq
curl -s https://ws.spade.my.id/health | jq
curl --fail --silent --show-error https://admin.spade.my.id/admin-api/health | jq
```

Expected response shape:

```json
{"status":"ok","service":"api"}
```

Admin returns `service: "admin-api"`, but its health endpoint is liveness-only.
Verify the selected player API release's migration ledger and startup logs,
then authenticated admin `/me` and `/dashboard` reads. Neither a database ping,
task convergence, nor `depends_on` establishes shared-schema readiness.

Check Swarm task health:

```bash
docker stack services 7spade
docker stack ps 7spade --no-trunc
```

For the agreed five-application stack, expect `api` and `ws` at `3/3`; `web`,
`admin-api`, `admin-web`, and the datastores should each converge to `1/1`.
The [admin smoke checklist](./admin.md#smoke-checklist) includes cookie rotation,
logout, MFA/permission/CSRF negative tests, and external direct-port checks.

## Logs

```bash
docker service logs -f 7spade_api
docker service logs -f 7spade_ws
docker service logs -f 7spade_admin-api
docker service logs -f 7spade_admin-web
```

## Resource Usage

```bash
docker stats --no-stream
```

## Database Connections

```bash
cid=$(docker ps -q -f name=7spade_postgres)
docker exec -i "$cid" psql -U sevens -c "SELECT count(*) FROM pg_stat_activity WHERE datname='sevens';"
```

## PostgreSQL Backups

PostgreSQL backup and restore procedures live in [Database Backups](./database-backups.md). The production recommendation is a daily compressed `pg_dump`, one local retention window, and S3-compatible off-server storage through `rclone`.

PostgreSQL backups do not include S3 skin-asset bytes or environment secrets.
Admin recovery planning must separately protect the asset bucket and persistent
`ADMIN_MFA_ENCRYPTION_KEY`; losing that key makes existing encrypted MFA
enrollments unreadable. Retain the key matching the backed-up database and test
restores in isolation. See [Admin recovery](./admin.md#upgrades-and-rollback).

## Redis Persistence

Both Redis services run with AOF persistence in [`deployment/stack.yml`](../../deployment/stack.yml). Redis data is useful for live room continuity, presence, owner leases, and relay state, but it is not durable-critical like PostgreSQL.

If a Redis volume is lost, in-progress rooms may reset or rehydrate from the next reconnect/snapshot path. PostgreSQL remains the durable system of record for users, rooms, and completed game history.

## Upgrading

CI publishes images on matching pushes to `main`, tags matching `v*`, and manual
workflow dispatches. The agreed release workflow preflights all five service
names before any update, then updates `api`, `ws`, `web`, `admin-api`, and
`admin-web`, with the player API first for shared migrations. It does not create
services or apply new env files; first provisioning must be manual.

For manual upgrades, update the API first and verify its migrations before
starting/updating admin. After that gate, deploy the final stack:

```bash
cd /opt/7spade
export IMAGE_TAG=vX.Y.Z # replace with the intended tag available for all five images
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

The agreed stack applies `${IMAGE_TAG:-latest}` to all five application images.
Record/export the intended tag on every deploy, including env-only changes;
release image updates do not persist it in the server shell or stack file.

Database migrations are embedded in the API image and applied automatically on startup.

Image rollback does not undo schema changes. Review compatibility before rolling
back any service, and coordinate database recovery across player and admin
applications if a schema restore is required. Preserve the MFA encryption key
across upgrades. See [Admin upgrades and rollback](./admin.md#upgrades-and-rollback).

To force a service restart without changing the stack file:

```bash
docker service update --force --with-registry-auth 7spade_api
docker service update --force --with-registry-auth 7spade_ws
```

After editing `api.env`, `ws.env`, `admin-api.env`, or `admin-web.env`, re-run
`docker stack deploy` with the recorded `IMAGE_TAG`. A plain
`docker service update --force` is not enough because Swarm bakes `env_file`
contents into the service spec at deploy time. Never persist bootstrap
credentials in those files; use the [one-off procedure](./admin.md#one-off-bootstrap).

Teardown interrupts both player and admin access. Preserve the deployed tag and
runtime files, and verify the retained schema is compatible before redeploying.
If restoring or initializing a database, use the API-first admin provisioning
gate instead of starting everything together. To tear down and redeploy against
an already verified compatible database:

```bash
docker stack rm 7spade
# wait for tasks to drain, then:
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

`docker stack rm` removes services and networks but not named volumes, so `postgres_data`, `redis_data`, and `redis_ws_data` survive a teardown.
