# HTTP API Reference

Base URL: `http://localhost:8080` (local) — set by the `PORT` environment variable.

All authenticated endpoints require an `Authorization: Bearer <JWT>` header.

---

## Health

### `GET /health`

Returns service liveness status.

**Response**
```json
{ "status": "ok", "service": "api" }
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

#### `DELETE /auth/logout`

Revokes the current refresh token and clears the cookie. No body required. Always returns `204`.

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

**Provider notes**

| Provider | Identity | Token verification |
|----------|----------|--------------------|
| Google | `sub` from `id_token` | JWKS at `googleapis.com/oauth2/v3/certs` |
| GitHub | Numeric `id` from `GET /user` | No `id_token` — plain OAuth 2.0 |
| Telegram | `sub` from `id_token` | JWKS at `oauth.telegram.org/.well-known/jwks.json` |

---

## Rooms

All room endpoints require authentication.

### `POST /rooms`

Creates a new room.

**Request body**
```json
{
  "visibility": "public",
  "turn_timer_seconds": 60
}
```

`visibility` is `"public"` or `"private"`. `turn_timer_seconds` must be one of `30`, `60`, `90`, or `120`.

**Response**
```json
{ "id": "<room-id>", "invite_code": "<code>" }
```

### `GET /rooms`

Lists public rooms with `waiting` status.

**Response**
```json
[
  { "id": "...", "invite_code": "...", "player_count": 2, "turn_timer_seconds": 60 }
]
```

### `POST /rooms/:code/join`

Joins a room by invite code. Returns an error if the room is full (4 players) or not in `waiting` status.

**Response**
```json
{ "id": "<room-id>" }
```

### `GET /rooms/:id`

Returns a room's current status and player count.

**Response**
```json
{ "id": "...", "status": "waiting", "player_count": 3, "turn_timer_seconds": 60 }
```

Room `status` values: `waiting` → `in_progress` (when 4th player joins).

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
