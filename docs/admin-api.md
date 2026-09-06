# Admin API

The admin API is a separate control plane served locally at
`http://127.0.0.1:8082`. It does not share player JWTs or refresh cookies. The
admin web app is served at `http://localhost:3001` in Compose and
`http://localhost:5174` by Vite.

## Security Model

- `ADMIN_JWT_SECRET` signs administrator access tokens and must not equal the
  player `JWT_SECRET`.
- Refresh tokens use an HttpOnly cookie. Browser refresh and logout requests
  include the CSRF header produced by the admin frontend.
- `ADMIN_FRONTEND_ORIGIN` is one exact credentialed origin, not a wildcard.
- `ADMIN_MFA_ENCRYPTION_KEY` is persistent, high-entropy key material from which
  the service derives its encryption key. Back it up securely; changing or
  losing it makes enrolled MFA secrets unreadable.
- Protected endpoints require an admin access token. Most also require the
  permission shown by their route group below.
- Mutations are recorded in the admin audit log. Sensitive investigation data
  must not be copied into ordinary application logs.

Bootstrap the first administrator with `make bootstrap` from
`services/admin-api`; subsequent administrators are invited through the admin
application. See [Development](./development.md#admin-applications).

## Authentication and Sessions

Public endpoints:

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/auth/login` | Password login; may return an MFA challenge |
| `POST` | `/auth/mfa/challenge` | Complete login with TOTP or a recovery code |
| `POST` | `/auth/refresh` | Rotate the administrator session |
| `DELETE` | `/auth/logout` | Revoke the current session |
| `POST` | `/auth/invitations/accept` | Accept an administrator invitation |

Authenticated administrators can use `/me`, `/sessions`,
`DELETE /sessions/{id}`, `DELETE /sessions/others`, and the MFA enroll/confirm
endpoints. Account status and session revocation are checked server-side.

## Protected Route Groups

| Area | Paths | Permissions |
|---|---|---|
| Dashboard | `/dashboard` | `dashboard.read` |
| Users | `/users`, `/users/{id}` | `users.read` |
| User moderation | suspension and display-name mutations under `/users/{id}` | `users.moderate` |
| Rooms | `/rooms`, `/rooms/{id}` | `rooms.read` |
| Games | `/games`, `/games/{id}` | `games.read` |
| Game annotations | `/games/{id}/flags`, `/games/{id}/notes` | `games.annotate` |
| Achievements | `/achievements` | `achievements.read` or `achievements.manage` |
| Achievement grants | `/users/{id}/achievements/{achievementID}/grant|revoke` | `achievements.entitlements` |
| Events | `/events` and lifecycle actions | `events.read` or `events.manage` |
| Skins | `/skins`, uploads, assets, and revisions | `skins.read` or `skins.manage` |
| Skin grants | `/users/{id}/skins/{skinId}/grant|revoke` | `skins.entitlements` |
| Admins and roles | `/admins`, `/roles`, `/permissions` | `admins.read` or `admins.manage` |
| Audit | `/audit-events`, `/audit-events/export` | `audit.read` or `audit.export` |

`POST /rooms/{id}/hidden-state` is an authenticated investigation endpoint. The
admin API calls the WS server with `WS_ADMIN_SERVICE_SECRET`, which must match
the WS service's `WS_INSPECTION_SECRET`. The returned state is redacted by the
WS inspection contract; this credential must never be sent to the browser.

## Deployment Status

Local Compose includes the admin API and frontend. The checked-in production
Swarm stack and image workflows do not currently build or deploy them. Do not
assume that enabling an admin hostname alone makes the control plane available;
production support requires images, service definitions, TLS/proxy routing,
secrets, bootstrap procedure, and network policy.
