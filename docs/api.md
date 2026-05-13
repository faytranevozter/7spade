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

Registers a new account. Hashes password with bcrypt and creates a row in `users`.

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
{ "token": "<jwt>", "refresh_token": "<refresh>" }
```

#### `POST /login`

Validates credentials and returns tokens.

**Request body**
```json
{ "email": "alice@example.com", "password": "hunter2" }
```

**Response**
```json
{ "token": "<jwt>", "refresh_token": "<refresh>" }
```

Returns `401` for incorrect credentials.

#### `POST /refresh`

Exchanges a valid refresh token for a new JWT.

**Request body**
```json
{ "refresh_token": "<refresh>" }
```

**Response**
```json
{ "token": "<jwt>" }
```

---

### OAuth

#### `GET /auth/google`

Redirects to Google OAuth consent. The handler sets a short-lived `oauth_state_google` cookie (10 minutes, HttpOnly, SameSite=Lax) used to validate the callback. Requires `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, and `GOOGLE_OAUTH_REDIRECT_URL` to be configured; otherwise returns `503`.

#### `GET /auth/google/callback`

Exchanges the authorization code for a Google access token, fetches the user's profile, and upserts the user in PostgreSQL (matched first by `(provider, provider_user_id)`, then by `email`). On success, the browser is redirected to `${FRONTEND_URL}/auth/callback#provider=google&jwt=<jwt>&refresh_token=<refresh>`. On failure, the redirect contains `error=<code>` and no tokens.

#### `GET /auth/github`

Same flow as `/auth/google` but for GitHub. Requires `GITHUB_OAUTH_CLIENT_ID`, `GITHUB_OAUTH_CLIENT_SECRET`, and `GITHUB_OAUTH_REDIRECT_URL`. Sets an `oauth_state_github` cookie.

#### `GET /auth/github/callback`

Exchanges the code, fetches `/user` and (if needed) `/user/emails` to find the primary verified email, then upserts the user and redirects the SPA as above.

#### `POST /auth/telegram`

Accepts a Telegram Login Widget payload, verifies the HMAC hash against the bot token, upserts the user, and returns a JWT.

**Request body** — Telegram login widget fields (including `hash`)

**Response**
```json
{ "token": "<jwt>", "refresh_token": "<refresh>" }
```

Returns `401` for invalid or expired Telegram payloads.

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
