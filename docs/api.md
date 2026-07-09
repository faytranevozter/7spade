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
  "display_name": "Alice",
  "username": "alice"
}
```

`username` must be 3–32 characters, lowercase letters, numbers, or underscores only.

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
> Native clients should store both tokens securely. The same body form is
> accepted by `DELETE /auth/logout` to revoke a native session.

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

### Password Reset & Email Verification

Reset and verification tokens are cryptographically random; only their SHA-256
hash is stored in Redis (single-use, with a TTL), so a store leak can't be
replayed. Emails are sent via the `EmailSender` abstraction — SMTP when
`SMTP_HOST` is configured, otherwise a dev sender that logs the link to the API
console. All links use the web app URL (`FRONTEND_URL`); native clients handle
the same `token` via deep-linked screens. The email-sending endpoints are
rate-limited per email (fixed 1-hour window): `forgot-password` 3/hr,
`resend-verification` 5/hr. A limited request still returns its normal status
(enumeration-safe) but sends no email.

#### `POST /auth/forgot-password`

Starts a password reset. **Always returns `200`** (even for unknown or
OAuth-only accounts) to prevent email enumeration. A reset link is emailed only
when a matching password account exists. The token expires in 15 minutes and is
single-use. Rate-limited to 3 emails/hour per email address.

```json
// request
{ "email": "alice@example.com" }
// response (always)
{ "message": "If an account exists, a reset link has been sent." }
```

#### `POST /auth/reset-password`

Consumes a reset token, sets the new bcrypt password hash, and **revokes all of
the user's refresh tokens** (logging out every session). `password` must be at
least 8 characters.

```json
// request
{ "token": "<token-from-email>", "password": "newpassword1" }
```

Returns `400` for an invalid, expired, or already-used token.

#### `POST /auth/verify-email`

Consumes a verification token and marks the account's email verified. Tokens
expire in 24 hours and are single-use.

```json
// request
{ "token": "<token-from-email>" }
```

Returns `400` for an invalid or expired token.

#### `POST /auth/resend-verification` *(authenticated)*

Re-sends the verification email for the authenticated user. No-op when already
verified or for guests. Always returns `204` so it leaks no account state.
Rate-limited to 5 emails/hour per email address.

> Verification is **soft**: unverified users can still play. Clients surface a
> dismissible banner (driven by `email_verified` on `GET /me`) prompting
> verification.

---

## Profile

### `GET /me` *(authenticated)*

Returns account information for the authenticated session. Registered users get
full profile data including linked providers; guests get a minimal response.

**Response (registered user)**
```json
{
  "user_id": "uuid",
  "username": "alice",
  "display_name": "Alice",
  "avatar_url": "https://...",
  "created_at": "2024-01-01T00:00:00Z",
  "is_guest": false,
  "email_verified": true,
  "providers": [
    { "provider": "google", "avatar_url": "https://...", "created_at": "2024-01-01T00:00:00Z" }
  ]
}
```

**Response (guest)**
```json
{
  "display_name": "Alice",
  "is_guest": true,
  "providers": []
}
```

### `PATCH /me` *(authenticated)*

Updates the authenticated (registered) user's display name. Guests are rejected with `401`.

The backend persists the new name to `users.display_name` and **re-issues the access JWT** carrying the new name (the display name is embedded in the JWT, which the WS server reads to label the player's seat). The refresh token is **not** rotated.

**Request body**
```json
{ "display_name": "New Name" }
```

`display_name` is trimmed and must be 1–50 characters.

**Response**
```json
{ "jwt": "<re-issued-access-token>" }
```

Returns `400` for an empty/over-length name, `401` for guests or an invalid token.

> **Caveat:** a rename does not relabel the player's seat in an *in-progress* WS game — the seat name is captured from the JWT at connection time. It applies to the next connection/game.

---

## Rooms

Creating and joining rooms require authentication. Listing public rooms and fetching a room by ID are public.

### `POST /rooms` *(authenticated)*

Creates a new room.

**Request body**
```json
{
  "name": "My Room",
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "min_elo": 800,
  "max_elo": 1200,
  "game_mode": "custom",
  "max_players": 6,
  "deck_count": 2,
  "scoring_mode": "flat",
  "team_mode": "2v2",
  "custom_scores": { "2": 1, "3": 1, "11": 10, "12": 10, "13": 10, "14": 20 }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | No | Room name, max 60 characters. Server assigns a default if omitted. |
| `visibility` | Yes | `"public"` or `"private"`. Forced to `"private"` when `practice_mode` is true. |
| `turn_timer_seconds` | Yes | One of `30`, `60`, `90`, or `120`. |
| `bot_difficulty` | No | `"easy"`, `"medium"`, or `"hard"`. Defaults to `"medium"`. |
| `practice_mode` | No | Solo-vs-bots. Forces private, excluded from lists, no history/stats. |
| `min_elo` / `max_elo` | No | Both required together. Non-negative, `min_elo <= max_elo`. Ignored in practice mode. |
| `game_mode` | No | `"classic"` (default) or `"custom"`. |
| `max_players` | No | 2–8. Defaults to 4. |
| `deck_count` | No | `1` (52 cards) or `2` (104 cards, double deck). Defaults to 1. |
| `scoring_mode` | No | `"rank_value"` (classic), `"flat"` (1 pt per card), or `"custom"` (per-rank). Defaults to `"rank_value"`. |
| `team_mode` | No | `"ffa"` (free for all) or `"2v2"` (teams of 2). Defaults to `"ffa"`. Only valid when `max_players` is 4 or 6. |
| `custom_scores` | No | Map of rank (2–14) to penalty points. Only used when `scoring_mode` is `"custom"`. Keys are stringified integers. |

**Response** `201 Created`
```json
{
  "id": "<room-id>",
  "invite_code": "<code>",
  "name": "My Room",
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "min_elo": 800,
  "max_elo": 1200,
  "game_mode": "custom",
  "max_players": 6,
  "deck_count": 2,
  "scoring_mode": "flat",
  "custom_scores": { "2": 1, "3": 1, "11": 10, "12": 10, "13": 10, "14": 20 },
  "team_mode": "2v2",
  "status": "waiting",
  "player_count": 1
}
```

Returns `409` with `{ "error": "You're already in another game", "active_room": {...} }` if the player is already in an active room.

### `GET /rooms`

Lists public rooms with `waiting` status.

**Response**
```json
[
  {
    "id": "...",
    "invite_code": "...",
    "name": "Room #1",
    "visibility": "public",
    "turn_timer_seconds": 60,
    "bot_difficulty": "medium",
    "practice_mode": false,
    "min_elo": null,
    "max_elo": null,
    "game_mode": "classic",
    "max_players": 4,
    "deck_count": 1,
    "scoring_mode": "rank_value",
    "team_mode": "ffa",
    "status": "waiting",
    "player_count": 2
  }
]
```

### `GET /rooms/{id}`

Returns a room's current status and player count.

**Response**
```json
{
  "id": "...",
  "invite_code": "ABC123",
  "name": "Room #1",
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "min_elo": null,
  "max_elo": null,
  "game_mode": "classic",
  "max_players": 4,
  "deck_count": 1,
  "scoring_mode": "rank_value",
  "team_mode": "ffa",
  "status": "waiting",
  "player_count": 3
}
```

Room `status` values: `waiting` → `in_progress` (when the host starts the
match) → `finished` (when the game ends). A `waiting` room is automatically
deleted once its last player leaves.

### `POST /rooms/{code}/join` *(authenticated)*

Joins a room by invite code. Returns an error if the room is full (4 players) or not in `waiting` status.

**Response**
```json
{ "id": "<room-id>", "invite_code": "ABC123", "status": "waiting", "player_count": 2 }
```

| Error | Status | Description |
|-------|--------|-------------|
| Room full | `409` | 4 players already |
| Not accepting | `409` | Room not in `waiting` status |
| Already in room | `409` | Player already joined this room |
| Kicked | `403` | Host removed the player from this room |
| Rating restricted | `403` | Player's rating outside the room's ELO range |
| Already in another | `409` | Player is in another active room (body includes `active_room`) |

### `POST /rooms/quick-play` *(authenticated)*

Finds or creates a public room for instant matchmaking. Joins the first available `waiting` public room, or creates one if none exist.

**Request body** (optional)
```json
{ "ranked": true }
```

When `ranked` is true, the player's rating is used for matchmaking (guests cannot use ranked). Rate-limited to one request per 3 seconds per user.

**Response** `200 OK` (joined existing) or `201 Created` (new room)
```json
{ "id": "<room-id>", "invite_code": "ABC123", "status": "waiting", "player_count": 2 }
```

Returns `409` if already in another active room, `429` if rate-limited.

### `GET /my/active-room` *(authenticated)*

Returns the `waiting` or `in_progress` room the player is currently in, or `null`.

**Response**
```json
{ "active_room": { "id": "...", "invite_code": "ABC123", "status": "waiting", "practice_mode": false } }
```

```json
{ "active_room": null }
```

### `GET /live-games`

Lists public rooms currently `in_progress` (for spectator / watch features).

**Response**
```json
{ "games": [...] }
```

---

## Game History

Requires authentication. Guest players' results are stored by display name only (no `user_id`).

### `GET /history` *(authenticated)*

Returns the authenticated player's past games, paginated. Guests are rejected with `401`.

**Query params**: `page` (default 1), `per_page` (default 10, max 50)

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

### `GET /games/{id}/replay` *(authenticated)*

Returns the full move list and initial deal for a finished game. Any authenticated user may read it (replays are shareable). Returns `404` when no replay data exists.

**Response**
```json
{
  "game_id": "...",
  "moves": [...],
  "initial_deal": [...]
}
```

---

## Stats & Leaderboard

### `GET /leaderboard`

Public paginated leaderboard of qualifying players.

**Query params**: `page`, `per_page` (max 50), `sort` (default: rating), `season` (`"all"` / `"active"` / `"current"` / specific season id)

**Response**
```json
{
  "entries": [...],
  "total": 100,
  "page": 1,
  "min_games": 5,
  "sort": "rating",
  "season": ""
}
```

### `GET /seasons`

Public list of seasons (newest first) for the leaderboard season selector.

**Response**
```json
{ "seasons": [...] }
```

### `GET /stats` *(authenticated)*

Returns the authenticated player's own stats. Guests are rejected with `401`. Returns zeroed counters if the player has no recorded games.

**Query params**: `season` (same options as leaderboard)

**Response**
```json
{
  "user_id": "...",
  "display_name": "Alice",
  "avatar_url": "https://...",
  "rating": 1000,
  "level": 1,
  "xp_for_next_level": 100,
  "xp_to_next_level": 100,
  "games_played": 0,
  "wins": 0,
  "win_rate": 0
}
```

### `GET /users/{id}/stats`

Public endpoint returning a specific user's stats. Returns `404` if the user has never played.

**Query params**: `season`

**Response**: same shape as `GET /stats`.

### `GET /users/{id}/achievements`

Public endpoint returning a player's earned achievements and the full catalog.

**Response**
```json
{
  "earned": [...],
  "catalog": [...]
}
```

### `GET /users/{id}/rating-history`

Public paginated history of a player's per-game rating changes.

**Query params**: `page` (default 1), `per_page` (default 20, max 50)

**Response**
```json
{
  "events": [...],
  "total": 50,
  "page": 1
}
```

---

## Friends

All friends endpoints require authentication and a registered (non-guest) account.

### `GET /friends` *(authenticated)*

Returns the caller's accepted friends and pending requests, enriched with live presence from Redis.

**Response**
```json
{
  "friends": [
    {
      "user_id": "...",
      "display_name": "Bob",
      "username": "bob",
      "avatar_url": "https://...",
      "status": "accepted",
      "online": true,
      "room_id": "..."
    }
  ]
}
```

`status` values: `accepted`, `incoming` (pending request from them), `outgoing` (pending request to them).

### `GET /users/search` *(authenticated)*

Searches registered users by partial username or display name for the add-friend flow. Excludes the caller and blocked relationships. Rate-limited to 30 requests/minute per user.

**Query params**: `q` (minimum 2 characters)

**Response**
```json
{ "results": [...] }
```

### `POST /friends/requests` *(authenticated)*

Sends a friend request. Resolves the target by `user_id` or exact `username`. If the target already sent the caller a request, it's auto-accepted.

**Request body**
```json
{ "user_id": "<uuid>" }
```
or
```json
{ "username": "bob" }
```

**Response**
```json
{ "status": "pending" }
```

| Error | Status |
|-------|--------|
| Adding yourself | `400` |
| Blocked | `403` |
| User not found | `404` |

### `POST /friends/requests/{userId}/accept` *(authenticated)*

Accepts an incoming friend request.

**Response**: `204 No Content`

Returns `404` if no pending request from that user.

### `DELETE /friends/{userId}` *(authenticated)*

Removes a friendship, cancels a pending request, or declines an incoming request.

**Response**: `204 No Content`

### `POST /friends/{userId}/block` *(authenticated)*

Blocks a user. Removes any existing friendship or pending request and prevents future requests.

**Response**: `204 No Content`

Returns `400` if trying to block yourself.

---

## Internal Endpoints

These service-to-service endpoints are called by the WebSocket server, not by
browsers, and are intended for the docker-internal network. `INTERNAL_API_SECRET`
is required (the API fails fast at startup if it is unset). Each request must
carry a matching `X-Internal-Secret` header; otherwise the API responds `401`.

### `POST /internal/games`

Persists a completed game and its per-player results. Guest players are stored
by display name only (no `user_id`).

**Response** `201 Created`
```json
{ "game_id": "<uuid>", "deltas": [...] }
```

### `POST /internal/rooms/{id}/status`

Updates a room's lifecycle status. Body: `{ "status": "in_progress" }` or
`{ "status": "finished" }` or `{ "status": "waiting" }`. Only forward transitions are allowed
(`waiting → in_progress → finished`).

**Response**: `204 No Content`

### `DELETE /internal/rooms/{id}/players/{userId}`

Drops a player's membership row when they leave the lobby. Idempotent — removing
a player who is already gone is not an error. Deletes the room when its last
`waiting`-phase player leaves.

**Response**: `204 No Content`

### `POST /internal/rooms/{id}/kick/{userId}`

Removes a player and records the kick so they cannot rejoin the room. Called by
the WS service when the host kicks someone.

**Response**: `204 No Content`

### `POST /internal/rooms/reconcile`

Receives the set of room IDs the WS server currently tracks in memory and
deletes presence-less `waiting` rooms (orphaned lobbies). Body:
`{ "active_room_ids": ["...", "..."] }`. Only `waiting` rooms that are absent
from the set **and** older than a short TTL (2 minutes) are removed.

**Response**
```json
{ "deleted": 1 }
```
