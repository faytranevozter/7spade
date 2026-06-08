# HTTP API Reference

Base URL: `http://localhost:8080` (local) — set by the `PORT` environment variable.

The Go API entry point is `services/api/cmd/api`; application code lives under `services/api/internal/*`. Database migrations are embedded from `services/api/internal/database/migrations/` and run automatically on startup.

All authenticated endpoints require an `Authorization: Bearer <JWT>` header.

---

## Health

### `GET /health`

Returns service liveness status and dependency reachability.

**Response**
```json
{ "status": "ok", "service": "api", "dependencies": { "postgres": "ok", "redis": "ok" } }
```

---

## Authentication

### Guest Login

#### `POST /guest`

Issues a short-lived JWT without creating a user account.

**Request body**
```json
{ "display_name": "Alice" }
```

**Response**
```json
{ "token": "<jwt>" }
```

JWT payload includes `sub` (random UUID), `display_name`, `is_guest: true`, and `exp`. No database write occurs.

---

### Email / Password

#### `POST /register`

Registers a new account. Hashes password with bcrypt, creates a row in `users`, and sets the refresh token as an HttpOnly cookie.

**Request body**
```json
{
  "email": "alice@example.com",
  "password": "hunter2",
  "display_name": "Alice"
}
```

**Response**
```json
{ "jwt": "<access-token>" }
```

The `refresh_token` is set as an `HttpOnly; SameSite=Strict` cookie (30-day TTL). It is never included in the JSON body.

#### `POST /login`

Validates credentials, returns an access JWT, and sets a new refresh token cookie.

**Request body**
```json
{ "email": "alice@example.com", "password": "hunter2" }
```

**Response**
```json
{ "jwt": "<access-token>" }
```

Returns `401` for incorrect credentials.

#### `POST /refresh`

Rotates the refresh token and issues a new access JWT. Reads the `refresh_token` cookie automatically — no request body required. The old refresh token is immediately revoked and a new one is set as a cookie.

**Response**
```json
{ "jwt": "<access-token>" }
```

Returns `401` if the cookie is missing, invalid, or expired.

> **Native clients (no cookie jar).** When no `refresh_token` cookie is present,
> the handler accepts the token in the body instead, and echoes a rotated token
> back in the body (web clients keep using the cookie and ignore this field):
>
> ```json
> // request:  { "refresh_token": "<token>" }
> // response: { "jwt": "<access-token>", "refresh_token": "<rotated-token>" }
> ```
>
> The mobile app persists both tokens in `expo-secure-store`. The same body
> form is accepted by `DELETE /auth/logout` to revoke a native session.

#### `DELETE /auth/logout`

Revokes the current refresh token and clears the cookie. No body required (web). Always returns `204`. Native clients may pass `{ "refresh_token": "<token>" }` in the body to revoke a session that was stored outside a cookie.

---

### OAuth / OIDC

All three providers follow the same Authorization Code + PKCE flow.

#### `GET /auth/{provider}/url`

`{provider}` = `google` | `github` | `telegram`

Generates a PKCE `code_verifier` + `code_challenge`, stores `{state → code_verifier}` in Redis (10-minute TTL), and returns the provider's authorization URL.

**Response**
```json
{ "url": "https://accounts.google.com/o/oauth2/v2/auth?...", "state": "<opaque>" }
```

Returns `503` if the provider is not configured.

> **Native clients.** Pass an optional `?redirect_uri=` query param (restricted
> to the `sevenspade://` / `exp://` deep-link schemes). It's stored with the PKCE
> state and replayed verbatim in the token exchange, so the provider redirects
> back into the app. Web omits it and uses the provider's configured default.

#### `POST /auth/{provider}/callback`

Validates the `state` against Redis (one-time — entry is deleted on use), exchanges the code + `code_verifier` for provider tokens, verifies the `id_token` via JWKS (Google, Telegram) or calls the GitHub user API, upserts `users` + `user_providers`, issues an app JWT, and sets a new refresh token cookie.

**Request body**
```json
{ "code": "<authorization-code>", "state": "<state-from-url-step>" }
```

**Response**
```json
{ "access_token": "<app-jwt>" }
```

The `refresh_token` is set as an HttpOnly cookie. Returns `401` for invalid/expired state, `502` for provider errors.

> **Native clients.** Include the same `redirect_uri` used in the URL step in the
> request body; the response also includes a `refresh_token` field (since native
> has no cookie jar):
>
> ```json
> // request:  { "code": "...", "state": "...", "redirect_uri": "sevenspade://auth/callback" }
> // response: { "access_token": "<app-jwt>", "refresh_token": "<token>" }
> ```

**Provider notes**

| Provider | Identity | Token verification |
|----------|----------|--------------------|
| Google | `sub` from `id_token` | JWKS at `googleapis.com/oauth2/v3/certs` |
| GitHub | Numeric `id` from `GET /user` | No `id_token` — plain OAuth 2.0 |
| Telegram | `sub` from `id_token` | JWKS at `oauth.telegram.org/.well-known/jwks.json` |

---

## Profile

### `PATCH /me`

Updates the authenticated (registered) user's display name. Requires `Authorization: Bearer <JWT>`. Guests are rejected with `401`.

The backend persists the new name to `users.display_name` and **re-issues the access JWT** carrying the new name (the display name is embedded in the JWT, which the WS server reads to label the player's seat). The refresh token is **not** rotated — the name isn't stored in it — so the same flow works for web (cookie) and native (body) clients without touching the refresh session.

**Request body**
```json
{ "display_name": "New Name" }
```

`display_name` is trimmed and must be 1–50 characters.

**Response**
```json
{ "jwt": "<re-issued-access-token>" }
```

The client must swap this token into its session (web: `AuthProvider` state + `sessionStorage`; native: `expo-secure-store`) so subsequent API calls and game connections use the new name. Returns `400` for an empty/over-length name, `401` for guests or an invalid token.

> **Caveat:** a rename does not relabel the player's seat in an *in-progress* WS game — the seat name is captured from the JWT at connection time. It applies to the next connection/game.

---

## Rooms

Creating and joining rooms require authentication. Listing public rooms and fetching a room by ID are public.

### `POST /rooms`

Creates a new room.

**Request body**
```json
{
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false
}
```

`visibility` is `"public"` or `"private"`. `turn_timer_seconds` must be one of `30`, `60`, `90`, or `120`. `bot_difficulty` is optional and defaults to `"medium"`; allowed values are `"easy"`, `"medium"`, and `"hard"`. `practice_mode` is optional and defaults to `false`; when `true` the room is a solo-vs-bots practice game and `visibility` is forced to `"private"` (you may omit `visibility`). Practice rooms are excluded from `GET /rooms` and `GET /live-games`, and their results are never saved to game history or stats.

**Response**
```json
{
  "id": "<room-id>",
  "invite_code": "<code>",
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "status": "waiting",
  "player_count": 1
}
```

### `GET /rooms`

Lists public rooms with `waiting` status.

**Response**
```json
[
  { "id": "...", "invite_code": "...", "player_count": 2, "turn_timer_seconds": 60, "bot_difficulty": "medium" }
]
```

### `POST /rooms/{code}/join`

Joins a room by invite code. Returns an error if the room is full (4 players) or not in `waiting` status.

**Response**
```json
{ "id": "<room-id>", "invite_code": "ABC123", "status": "waiting", "player_count": 2 }
```

### `GET /rooms/{id}`

Returns a room's current status and player count.

**Response**
```json
{ "id": "...", "invite_code": "ABC123", "visibility": "public", "status": "waiting", "player_count": 3, "turn_timer_seconds": 60, "bot_difficulty": "medium" }
```

Room `status` values: `waiting` → `in_progress` (when the host starts the
match) → `finished` (when the game ends). A `waiting` room is automatically
deleted once its last player leaves, so it stops appearing in the public list.

---

## Internal Endpoints

These service-to-service endpoints are called by the WebSocket server, not by
browsers, and are intended for the docker-internal network. When
`INTERNAL_API_SECRET` is configured, each request must carry a matching
`X-Internal-Secret` header; otherwise the API responds `401`. When the secret is
unset, the guard is disabled (all requests pass) for backward compatibility.

### `POST /internal/games`

Persists a completed game and its per-player results. Guest players are stored
by display name only (no `user_id`).

### `POST /internal/rooms/{id}/status`

Updates a room's lifecycle status. Body: `{ "status": "in_progress" }` or
`{ "status": "finished" }`. Only forward transitions are allowed
(`waiting → in_progress → finished`).

### `DELETE /internal/rooms/{id}/players/{userId}`

Drops a player's membership row when they leave the lobby. Idempotent — removing
a player who is already gone is not an error. Deletes the room when its last
`waiting`-phase player leaves.

### `POST /internal/rooms/reconcile`

Receives the set of room IDs the WS server currently tracks in memory and
deletes presence-less `waiting` rooms (orphaned lobbies). Body:
`{ "active_room_ids": ["...", "..."] }`. Only `waiting` rooms that are absent
from the set **and** older than a short TTL (2 minutes) are removed.

**Response**
```json
{ "deleted": 1 }
```

---

## Game History

Requires authentication. Guest players' results are stored by display name only (no `user_id`).

### `GET /history`

Returns the authenticated player's past games, paginated.

**Query params**: `page`, `per_page`

**Response**
```json
{
  "games": [
    {
      "game_id": "...",
      "room_id": "...",
      "started_at": "2024-01-01T10:00:00Z",
      "finished_at": "2024-01-01T10:30:00Z",
      "penalty_points": 15,
      "rank": 1,
      "is_winner": true
    }
  ],
  "total": 42,
  "page": 1
}
```
