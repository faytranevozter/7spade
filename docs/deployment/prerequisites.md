# Prerequisites

| Requirement | Minimum |
|---|---|
| VPS CPU | 1 vCPU |
| RAM | 1 GB minimum; more recommended for 3 `api` and 3 `ws` replicas |
| Disk | 20 GB SSD |
| OS | Ubuntu 22.04 LTS or Debian 12+ |
| Docker | 24+ with Swarm mode enabled |
| Domain | One domain with three A records pointing to the VPS |
| Ports | 80 and 443 open to the internet |

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

Create three DNS A records pointing to the VPS IP:

```text
spade.example.com       -> <VPS IP>
api-spade.example.com   -> <VPS IP>
wsspade.example.com     -> <VPS IP>
```

Current production uses:

| Subdomain | Service |
|---|---|
| `spade.fahrur.my.id` | Web frontend |
| `api-spade.fahrur.my.id` | HTTP API |
| `wsspade.fahrur.my.id` | WebSocket server |
