# Scaling

The current production stack in [`deployment/stack.yml`](../../deployment/stack.yml) already runs multiple `api` and `ws` replicas:

| Service | Replicas |
|---|---:|
| `api` | 3 |
| `ws` | 3 |

The static `web` service remains a single nginx container, and PostgreSQL/Redis remain single-instance services on the VPS.

## Single-Server Capacity Notes

| Concern | Practical limit |
|---|---|
| Concurrent WS connections | Depends on VPS memory and Go goroutine overhead |
| PostgreSQL connections | Bound by Postgres `max_connections` and API replicas |
| Redis room snapshots | Memory-bound |

Scale beyond the current VPS when connection count, database load, or availability requirements exceed what one host can support.

## Horizontal WS Scaling Owner + Relay Model

The WS service uses an owner + Redis pub/sub relay model so multiple replicas can serve one room's players behind Swarm's round-robin VIP with no sticky sessions. See the [multi-replica WebSocket architecture](../architecture.md#horizontal-scaling--multi-replica-websocket-owner--relay) for the full diagram.

- `WS_REDIS_URL` points every `ws` replica to `redis-ws`.
- One replica owns each room through a Redis lease and fencing token.
- Non-owner replicas act as edges: they hold sockets, forward inbound client messages to the owner, and deliver outbound messages to local sockets.
- The owner applies state changes, runs timers, controls bots, saves game results, and publishes rendered room views.
- Failover is checkpoint-based: snapshots are persisted after moves, and another replica can claim and rehydrate a room after owner lease expiry.
- Orphan-room reconciliation runs cluster-wide instead of once per replica.

## Scaling `ws`

For the current stack, `redis-ws` and `WS_REDIS_URL` are already expected. To change replica count, edit `deployment/stack.yml` and redeploy:

```bash
cd /opt/7spade
docker stack deploy --with-registry-auth -c stack.yml 7spade
```

Or scale the running service directly for a temporary change:

```bash
docker service scale 7spade_ws=3
```

Verify convergence:

```bash
docker stack services 7spade
docker service logs 7spade_ws
```

nginx does not need stickiness changes. Swarm round-robins the published `8081` port across tasks, and room state is coordinated through `redis-ws`.

## Future Scaling Options

| Area | Next step |
|---|---|
| PostgreSQL | Move to a managed database such as Supabase, Neon, or RDS |
| Redis | Move relay/cache Redis to managed Redis or a dedicated node |
| Web frontend | Serve static assets from a CDN or static host |
| API | Add replicas and tune database connection pooling |
| WS | Run more replicas, then move beyond a single Swarm node if host limits are reached |
