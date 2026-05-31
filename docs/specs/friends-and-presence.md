# Spec: Friends & Presence

Status: Implemented
Owner: —
Related: [Architecture](../architecture.md) · [HTTP API](../api.md) · [WebSocket Protocol](../websocket.md) · [Multi-Provider OAuth](../multi-provider-oauth.md)

## 1. Overview

Let registered players add each other as friends, see who's online, and invite
friends straight into a room. Friendships are durable (Postgres); presence is
ephemeral (Redis), driven by WebSocket connect/disconnect. This turns the lobby
from a list of anonymous public rooms into a social hub.

### Goals

- Send, accept, and remove friend requests.
- See which friends are currently online (and ideally, in a game).
- Invite a friend to a room you're in via a shareable link / in-app prompt.

### Non-goals

- Friends for guests (no durable identity).
- A full chat/DM system (out of scope; emotes already cover in-game expression).
- Blocking/abuse tooling beyond a basic block (kept minimal in v1).
- Cross-app social graph import.

## 2. Data Model

### Migration `006_friendships.sql`

```sql
CREATE TABLE IF NOT EXISTS friendships (
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'accepted' | 'blocked'
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (requester_id, addressee_id),
    CHECK (requester_id <> addressee_id)
);

CREATE INDEX IF NOT EXISTS idx_friendships_addressee ON friendships(addressee_id);
```

A friendship is one directed row (requester → addressee). An accepted friendship
is queried in both directions (`requester_id = me OR addressee_id = me AND
status = 'accepted'`). `blocked` is owned by `requester_id` (the blocker).

### Presence (Redis)

Presence is not durable. The WS service maintains an online set keyed by
`user_id` with a short TTL refreshed by a heartbeat:

- On WS connect (lobby or game), `SADD`/`SET presence:<user_id>` with a TTL
  (e.g. 60s), refreshed periodically while connected.
- On disconnect / TTL expiry, the user drops offline.
- Optionally store the user's current `room_id` so friends can see "in a game"
  and jump to spectate.

## 3. API (`services/api`)

All friend endpoints require a registered (non-guest) session, mirroring
`/history` and `/stats`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/friends` | List accepted friends + incoming/outgoing pending requests, each with presence |
| `POST` | `/friends/requests` | Send a request (`{ "user_id" }` or `{ "username" }`) |
| `POST` | `/friends/requests/:id/accept` | Accept an incoming request |
| `DELETE` | `/friends/requests/:id` | Decline / cancel a pending request |
| `DELETE` | `/friends/:userId` | Remove an existing friend |
| `POST` | `/friends/:userId/block` | Block a user (optional v1) |

Presence is read from Redis at request time and merged into the friends list
(`online`, optional `room_id`). Each friend entry also carries `username`
alongside `display_name`. Repository functions follow the existing
free-function-with-`*sql.DB` pattern; handlers mirror `StatsHandler` /
`HistoryHandler` (claims extraction, guest rejection).

### Discovery

Adding a friend is by exact **username** — a unique, lowercase handle
(`^[a-z0-9_]{3,32}$`) distinct from the human-facing (non-unique) display name.
The client normalizes input to lowercase; the server normalizes again and looks
up `users.username` (unique index), so there's exactly one match or none. This
replaced the original add-by-display-name flow, which had to return `409` on
name collisions and force the user to add by profile id. A fuzzy user search is
still future work; exact-match avoids building a search index and the privacy
questions that come with enumeration.

## 4. Presence Wiring (`services/ws`)

The WS service already owns every live connection, so it's the natural presence
writer:

- On `joinRoom` / spectator connect: mark the user online in Redis (the WS
  service already holds a Redis client for room snapshots).
- A periodic heartbeat refreshes the TTL while connected.
- On disconnect: best-effort clear (and let TTL expire as a backstop).
- The API only **reads** presence — no new write path or coupling.

Presence intentionally has no historical record; it's a live snapshot.

## 5. Frontend (`web`)

- `api/friends.ts` client for the endpoints above.
- A **Friends panel** in the lobby (`LobbyPage`): accepted friends with online
  dots (and "in game" + a Watch link when `room_id` is present, tying into
  [Spectator Mode](./spectator-mode.md)), plus a pending-requests section with
  accept/decline. Each row shows the display name with the `@username` beneath
  it.
- "Add friend" by **username** (lowercased as the user types); a small badge on
  the header nav when there are incoming requests.
- Registration collects a username (`RegisterPage`) in addition to a display
  name; the field is validated client-side (`^[a-z0-9_]{3,32}$`) and forced to
  lowercase.
- "Invite" from the waiting room: copies/sends the room invite link to a friend
  (in-app prompt; delivery is an Open Question).
- Presence refreshes on navigation / refetch (no realtime push in v1).

## 6. Edge Cases

- **Guests**: excluded — no `user_id`, so no friendships and no presence row;
  friend UI is hidden for guest sessions.
- **Duplicate / reverse requests**: if A requests B while B already requested A,
  auto-accept (both intentions present). The composite PK + direction handling
  must reconcile this.
- **Self-request**: rejected by the `CHECK` constraint and handler validation.
- **Deleted user**: `ON DELETE CASCADE` removes their friendship rows.
- **Blocked users**: cannot send requests or appear in each other's lists.
- **Stale presence**: TTL expiry handles ungraceful disconnects; a friend may
  briefly appear online after a crash until the TTL lapses.
- **Usernames**: unique and lowercase, so add-by-username always resolves to a
  single user (or none). Display names stay non-unique and are only cosmetic.

## 7. Testing

- Repository: request → accept → list (bidirectional), remove, decline,
  duplicate/reverse auto-accept, self-request rejected, block.
- Handler: all endpoints reject guests (401); bad ids → 400; not-found → 404.
- Presence: WS connect marks online; disconnect/TTL drops offline; API merges
  presence into the friends list (mock Redis).
- Web: friends panel renders online/offline + pending; add-by-username flow; invite
  link.
- Run `make -C services/api test`, `make -C services/ws test`, and
  `cd web && npm test && npm run lint && npm run build`.

## 8. Rollout

1. Ship migration `006` (friendships).
2. API: friend endpoints (read presence from Redis).
3. WS: presence writes on connect/disconnect + heartbeat.
4. Frontend: friends panel, add-by-name, invite.
5. No breaking changes; guests are simply unaffected.

## 9. Open Questions / Future Work

### Implementation notes (v1 as shipped)

- **Migration is `008_friendships.sql`** (006/007 were taken by achievements).
- **Presence key**: the WS service writes `presence:user:<id>` (value = current
  `room_id`, or "") with a 60s TTL, refreshed by a per-connection 25s heartbeat;
  the API reads it via `MGET`. On disconnect the key is **not** cleared eagerly —
  it lapses via TTL — which avoids flapping offline during the lobby
  reconnect-grace window. Spectators count as online too.
- **One DELETE endpoint** `DELETE /friends/:userId` covers decline, cancel, and
  unfriend (the repository removes any non-blocked relation in either
  direction), rather than the separate `/friends/requests/:id` decline route in
  the table above.
- **Add-by-username** resolves via `GetUserByUsername` against the unique
  `users.username` (lowercase) column added in `009_usernames.sql`. The handler
  normalizes the input (trim + lowercase) and returns `404 No player with that
  username` when there's no match. Email registration collects a user-chosen
  username; OAuth sign-ups auto-generate one (provider handle → email local-part
  → display name, with a `_N` suffix on collision) and keep it stable across
  later sign-ins.
- **Header badge**: the incoming-request count is shown on the existing "Lobby"
  nav link (polled every 15s) rather than a dedicated `/friends` route.
- **Invite link**: the waiting room copies `/lobby?invite=CODE`; the lobby reads
  `?invite=` to prefill and open the join dialog (in-app delivery).
- **`empty-body` handling**: `apiRequest` now tolerates 204/empty responses
  (accept/remove return 204).

### Still open

- **Invite delivery** — in-app + shareable link only (no email/push).
- **Realtime presence push** — still refetch-on-poll, no WS push.
- **User search** — exact username / user-id only; no fuzzy search.
- **Username editing** — usernames are set at registration (chosen for email
  sign-up, auto-generated for OAuth) and not yet editable; a profile edit
  (`GET`/`PATCH /me`) is planned follow-up work.
- **Richer blocking / reporting** — `POST /friends/:userId/block` exists but has
  no management UI yet.
- **"Play with friends" matchmaking** — private room auto-creation from the
  friends panel.
