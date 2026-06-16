# Operations

## Health Checks

Both services expose `/health` endpoints:

```bash
curl -s https://api-spade.fahrur.my.id/health | jq
curl -s https://wsspade.fahrur.my.id/health | jq
```

Expected response shape:

```json
{"status":"ok","service":"api"}
```

Check Swarm task health:

```bash
docker stack services 7spade
docker stack ps 7spade --no-trunc
```

For the current stack, expect `api` and `ws` to converge to `3/3`; the other services should converge to `1/1`.

## Logs

```bash
docker service logs -f 7spade_api
docker service logs -f 7spade_ws
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

Create `/opt/backups`, then schedule a daily `pg_dump`. Swarm has no `exec` subcommand, so resolve the running task's container id and use `docker exec`:

```bash
0 3 * * * cid=$(docker ps -q -f name=7spade_postgres) && docker exec -i "$cid" pg_dump -U sevens sevens | gzip > /opt/backups/sevens-$(date +\%Y\%m\%d).sql.gz
```

Rotate old backups:

```bash
find /opt/backups -name "sevens-*.sql.gz" -mtime +30 -delete
```

Restore:

```bash
cid=$(docker ps -q -f name=7spade_postgres)
gunzip -c /opt/backups/sevens-20250101.sql.gz | docker exec -i "$cid" psql -U sevens sevens
```

## Redis Persistence

Both Redis services run with AOF persistence in [`deployment/stack.yml`](../../deployment/stack.yml). Redis data is useful for live room continuity, presence, owner leases, and relay state, but it is not durable-critical like PostgreSQL.

If a Redis volume is lost, in-progress rooms may reset or rehydrate from the next reconnect/snapshot path. PostgreSQL remains the durable system of record for users, rooms, and completed game history.

## Upgrading

CI publishes fresh images on pushes to `main`, tags matching `v*`, and manual workflow dispatches. To roll out the image tag referenced by `/opt/7spade/stack.yml`:

```bash
cd /opt/7spade
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

If using immutable tags, update `/opt/7spade/stack.yml` first.

Database migrations are embedded in the API image and applied automatically on startup.

To force a service restart without changing the stack file:

```bash
docker service update --force --with-registry-auth 7spade_api
docker service update --force --with-registry-auth 7spade_ws
```

After editing `api.env` or `ws.env`, re-run `docker stack deploy`. A plain `docker service update --force` is not enough because Swarm bakes `env_file` contents into the service spec at deploy time.

To tear down and redeploy:

```bash
docker stack rm 7spade
# wait for tasks to drain, then:
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

`docker stack rm` removes services and networks but not named volumes, so `postgres_data`, `redis_data`, and `redis_ws_data` survive a teardown.
