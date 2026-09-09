# Reverse Proxy And TLS

nginx terminates HTTP/TLS and connects to the Swarm-published ports through the
node's loopback address:

| Host | Upstream |
|---|---|
| `spade.my.id` | `127.0.0.1:3000` |
| `api.spade.my.id` | `127.0.0.1:8080` |
| `ws.spade.my.id` | `127.0.0.1:8081` |
| `admin.spade.my.id` (proposed) | `127.0.0.1:3001`, admin-web SPA and `/admin-api` proxy |

The canonical nginx config is [`deployment/nginx/7spade.conf`](../../deployment/nginx/7spade.conf).

The admin host is part of the [agreed deployment target](./admin.md), not a claim
that DNS/TLS or the infrastructure changes are already live. Confirm the selected
release includes its virtual host and the admin-web cookie-path rewrite before
installing the configuration. No public admin API hostname or port is needed.

The loopback upstream address in nginx does not make a Swarm published port
loopback-only. The agreed stack publishes frontend/player ports on the node. Block public
access to `3000`, `3001`, `8080`, `8081`, and `8082` with provider firewall rules
and appropriate host policy; expose only 80/443 for application traffic. Keep
`admin-api` port `8082` unpublished. Docker networking can bypass simple host
firewall rules. Confirm from an external client that direct ports are unreachable
on every node and over IPv4/IPv6 as applicable.

## Admin Proxy Requirements

The outer admin virtual host terminates TLS and forwards to admin-web, not
directly to admin-api. Its proxy location requires at least:

```nginx
client_max_body_size 6m;
location / {
    proxy_pass http://127.0.0.1:3001;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Inside admin-web, `/admin-api/` proxies to `http://admin-api:8082/` over the stack
network. Keep the trailing slash to strip the browser prefix. That location also
needs `client_max_body_size 6m` and `proxy_cookie_path /auth /admin-api/auth`.
The cookie rewrite must apply to both refresh-cookie issuance and deletion;
the CSRF cookie path `/` must remain unchanged. Otherwise password login can
appear successful while refresh/logout fail through the prefixed route.

Both image workflows must compile `VITE_ADMIN_API_URL=/admin-api`.
`ADMIN_FRONTEND_ORIGIN` must exactly match `https://admin.spade.my.id`, and
`ADMIN_SECURE_COOKIES=true` is required despite the private HTTP proxy hop.
Keep the admin-web security headers and startup CSP; set only the trusted asset
origin in `SKIN_ASSETS_ORIGIN`. Do not add wildcard image/connect sources or
publish `8082` to work around proxy errors.

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

After confirming all four hosts' DNS and installing the admin virtual host,
obtain certificates (the admin hostname remains proposed until provisioned):

```bash
sudo certbot --nginx \
  -d spade.my.id \
  -d api.spade.my.id \
  -d ws.spade.my.id \
  -d admin.spade.my.id
```

Certbot will add TLS directives, configure HTTP-to-HTTPS redirects, and register renewal.

Verify `https://admin.spade.my.id/admin-api/health`, then browser login, refresh,
logout, and MFA using the [admin smoke checklist](./admin.md#smoke-checklist).
A public health response alone does not verify schema readiness or cookies.

Verify renewal:

```bash
sudo certbot renew --dry-run
```
