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
| `POST` | `/auth/invitations/inspect` | Validate an invitation token from the request body and return its email, role, and expiry |

Authenticated administrators can use `/me`, `/sessions`,
`DELETE /sessions/{id}`, `DELETE /sessions/others`, and the MFA enroll/confirm
endpoints. Account status and session revocation are checked server-side.

Administrators with `admins.manage` create a 48-hour invitation from the
Administrators page and securely share the one-time link shown in that response.
The recipient opens `#/accept-invitation?token=...`, reviews the invited email
and role, then chooses a display name and password. Invitation credentials are
stored only as hashes and become invalid after acceptance, revocation, expiry,
or reissue. Administrators with `admins.read` can review invitation history;
those with `admins.manage` can revoke or reissue pending invitations. Reissue
rotates the credential and invalidates the previous link.

After signing in, a new administrator enrolls an authenticator from Security,
confirms a six-digit code, and stores the one-time recovery codes. Invitation
email delivery is not configured; the inviter must share the displayed link.

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
| Skins | `/skins`, `/skins/{id}`, uploads, assets, and revisions | `skins.read` for GET; `skins.manage` for mutations |
| Skin grants | `/users/{id}/skins/{skinId}/grant|revoke` | `skins.entitlements` |
| Admins and roles | `/admins`, `/roles`, `/permissions` | `admins.read` or `admins.manage` |
| Audit | `/audit-events`, `/audit-events/export` | `audit.read` or `audit.export` |
| Settings | `GET /settings`, `PUT /settings/{key}`; legacy `GET /settings/daily-login`, `PUT /settings/daily-login` | `settings.read` for GET; `settings.write` for PUT |

`GET /settings` returns an array of `{ "key": "room_creation", "enabled": true }`
objects. Accepted keys are `daily_login`, `new_registrations`, `guest_access`,
`room_creation`, `quick_play`, `new_game_starts`, `spectator_access`, and
`emotes`. Unknown update keys return `404`.

`PUT /settings/{key}` requires an `enabled` boolean and a non-empty `reason`:
`{ "enabled": false, "reason": "Maintenance window" }`. It returns the saved
key and enabled state. Missing or invalid fields return `400`; missing
permissions return `403`. Mutations retain the administrator session's CSRF
requirements. Each update is committed atomically with a
`setting.{key}.update` audit event containing the reason and before/after states.
New settings default enabled; migrations preserve existing values. The legacy
Daily Login endpoints operate on the same `daily_login` row.

`POST /rooms/{id}/hidden-state` is an authenticated investigation endpoint. The
admin API calls the WS server with `WS_ADMIN_SERVICE_SECRET`, which must match
the WS service's `WS_INSPECTION_SECRET`. The returned state is redacted by the
WS inspection contract; this credential must never be sent to the browser.

## User Detail

`GET /users/{id}` requires `users.read`. Email is omitted without
`users.sensitive.read`; linked provider names do not expose provider credentials.
The response retains `user`, `providers`, `stats`, `ratings`, `achievements`,
`skins`, `games`, and optional `room`.

- `stats` contains `games_played`, `wins`, `total_penalty`, and `xp` when a stats
  row exists; an empty object means unavailable, not zero.
- `achievements` entries contain `achievement_id`, `name`, `description`, `icon`,
  and `earned_at`, including earned achievements no longer enabled in the catalog.
- `skins` entries contain `id`, `name`, `skin_type`, `source`, `earned_at`,
  optional `revision_id`, and `asset_url`. Preview URLs use the owned revision,
  including disabled historical revisions, not the current catalog revision.
  Missing ownership revisions never fall forward to current artwork. Rule-earned
  ownership reports `source: "unlock_rule"`; other source values are retained.
  Empty asset keys or
  unconfigured asset storage produce an empty URL. Internal asset keys are not
  exposed. Reward metadata is loaded with set-based joins, not per-item queries.
- `ratings` and `games` each contain at most the latest 100 records, newest first.
  Their lengths are not lifetime totals.

The user dossier uses `?section=overview|skins|achievements|activity` inside the
admin hash route. Unknown sections render Overview. Collections support local
search, sorting, and 12-card pages; skins also filter by type and source. Activity
tables use 10-row pages within the latest-100 window. Game, room, and audit links
require their destination read permissions. Account details and existing
moderation remain in the sidebar. Ownership is read-only: this page does not
grant/revoke rewards, infer equipped state, or invent achievement progress.

## Skin Detail

`GET /skins/{id}` requires an admin bearer token and `skins.read` (having
`skins.manage` alone does not grant read access). It returns one skin object
directly, not a `{ "skins": [...] }` collection wrapper.

The response includes the same fields as a list entry: `id`, `skin_type`,
`name`, `description`, `asset_key`, `asset_url`, `is_starter`, `display_order`,
`enabled`, `catalog_visible`, `unlock_rules_locked`, `unlock_rules`, and
`revisions`. Unlock rules include their conditions; revisions include enabled
and disabled history ordered by descending version. Starter status and the
unlock-rule lock are read from the catalog and entitlement history. Disabled
or catalog-hidden skins are still readable by authorized administrators.
`asset_url` is populated only when asset storage is configured and `asset_key`
is non-empty, using the same public URL rules as `GET /skins`. Empty rules and
revisions may be `null`, as in the list response.

Responses: `200` for a skin, `401` for missing/invalid authentication, `403`
without `skins.read`, `404` with `{ "error": "Skin not found" }` for an unknown
or malformed ID, and `500` with `{ "error": "Failed to load skin" }` for a
storage failure.

The existing `docs/openapi.yaml` describes the player API on port 8080, not
this separate admin API on port 8082.

## Deployment Status

Local Compose includes the admin API and frontend. The checked-in production
Swarm stack and image workflows do not currently build or deploy them. Do not
assume that enabling an admin hostname alone makes the control plane available;
production support requires images, service definitions, TLS/proxy routing,
secrets, bootstrap procedure, and network policy.
