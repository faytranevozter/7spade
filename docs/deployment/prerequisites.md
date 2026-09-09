# Prerequisites

| Requirement | Minimum |
|---|---|
| VPS CPU | 1 vCPU |
| RAM | 1 GB minimum; size with headroom for 3 `api`, 3 `ws`, and the two single-replica admin services |
| Disk | 20 GB SSD |
| OS | Ubuntu 22.04 LTS or Debian 12+ |
| Docker | 24+ with Swarm mode enabled |
| Domain | One domain with four A records for the agreed player/admin target |
| Ports | 80 and 443 open to the internet |

The agreed Swarm stack publishes application ports `3000`, `3001`, `8080`, and
`8081` on the node/routing mesh, not loopback-only. Keep admin-api `8082`
unpublished. Use provider firewall policy and appropriate host rules to prevent
public access to all direct application ports, including `8082`; Docker can
bypass simple host firewall rules. External application traffic must enter
through nginx on 80/443 so TLS and proxy policy cannot be bypassed. Verify from
outside the VPS on all nodes and address families after deployment. Restrict SSH
and Swarm management access separately.

Install Docker and initialize Swarm on a fresh Ubuntu/Debian server:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# log out and back in to apply group change
docker swarm init
docker node ls
```

`docker swarm init` on a single VPS makes it a one-node manager, which is enough to run `docker stack deploy`. For a multi-node cluster, join additional workers with the token printed by `docker swarm init`.

## DNS

Create four DNS A records for the agreed target pointing to the VPS IP:

```text
spade.example.com       -> <VPS IP>
api-spade.example.com   -> <VPS IP>
wsspade.example.com     -> <VPS IP>
admin.spade.example.com -> <VPS IP>
```

Player production hosts and the proposed admin host:

| Subdomain | Service |
|---|---|
| `spade.my.id` | Web frontend |
| `api.spade.my.id` | HTTP API |
| `ws.spade.my.id` | WebSocket server |
| `admin.spade.my.id` (proposed) | Admin frontend with same-origin `/admin-api` |

Confirm the admin hostname before provisioning and cover it with TLS. Add AAAA
records only with working IPv6 routing and equivalent firewall controls. Follow
[Admin Deployment](./admin.md) for runtime secrets, all five operations URLs,
database/MFA-key backups, schema readiness, and manual first provisioning before
release auto-updates.
