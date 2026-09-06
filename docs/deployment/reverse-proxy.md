# Reverse Proxy And TLS

nginx terminates HTTP/TLS and connects to the Swarm-published ports through the
node's loopback address:

| Host | Upstream |
|---|---|
| `spade.my.id` | `127.0.0.1:3000` |
| `api.spade.my.id` | `127.0.0.1:8080` |
| `ws.spade.my.id` | `127.0.0.1:8081` |

The canonical nginx config is [`deployment/nginx/7spade.conf`](../../deployment/nginx/7spade.conf).

The loopback upstream address in nginx does not make a Swarm published port
loopback-only. The current stack publishes these ports on the node. Block public
access to `3000`, `8080`, and `8081` with host/provider firewall rules and expose
only 80/443. Confirm that an external client cannot reach the backend ports
directly.

## Install Config

Copy the config to the server:

```bash
scp deployment/nginx/7spade.conf <user>@<vps>:/tmp/7spade.conf
ssh <user>@<vps> 'sudo mv /tmp/7spade.conf /etc/nginx/sites-available/7spade'
```

If you are already on the server and have the repo checked out:

```bash
sudo cp deployment/nginx/7spade.conf /etc/nginx/sites-available/7spade
```

Enable the site:

```bash
sudo ln -s /etc/nginx/sites-available/7spade /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

The WebSocket host must include HTTP/1.1 upgrade headers:

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
```

These are already present in [`deployment/nginx/7spade.conf`](../../deployment/nginx/7spade.conf).

## TLS With Certbot

Install Certbot and the nginx plugin:

```bash
sudo apt install -y certbot python3-certbot-nginx
```

Obtain certificates:

```bash
sudo certbot --nginx \
  -d spade.my.id \
  -d api.spade.my.id \
  -d ws.spade.my.id
```

Certbot will add TLS directives, configure HTTP-to-HTTPS redirects, and register renewal.

Verify renewal:

```bash
sudo certbot renew --dry-run
```
