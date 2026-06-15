# Spec: Spectator Mode

Status: Implemented
Owner: —
Related: [WebSocket Protocol](../websocket.md) · [Architecture](../architecture.md) · [Player Stats & Leaderboard](./stats-and-leaderboard.md)

## 1. Overview

Let anyone watch a live game without taking a seat. A spectator opens a
read-only WebSocket to a room and receives **redacted** game state — the public
board, card counts, whose turn it is, and the eventual results — but never any
hidden information (no player's hand). Spectator entry points are deep-linkable,
notably from the leaderboard ("watch the #1 player").

### Goals

- Watch a live game in real time, read-only.
- Never leak hidden information (hands, face-down card identities mid-game).
- Don't disturb the game: spectators don't occupy seats, affect `can_start`,
  turn order, bot backfill, results, or rematch.

### Non-goals

- Spectator chat (text) — could be added later; out of scope for now.
  Spectator **emotes** are now implemented (see `docs/specs/emotes.md` §
  Spectator emotes and issue #47).
- Rewind / replay / time-travel (that's a separate replay feature).
- Spectating finished games beyond the live results screen.
- Spectating from the lobby phase (v1 starts spectating once a game is in
  progress; lobby spectating is optional, see Open Questions).

## 2. Connection Model

Spectators reuse the existing game WebSocket with a role marker:

```
ws://host/ws?room_id=X&token=JWT&role=spectator
```

- `token` still required (auth), but the spectator is **not** added to
  `room.players` and takes no seat. A separate `room.spectators` slice holds
  their connections.
- A spectator connecting to a non-existent / not-yet-started room is rejected (or
  held until start, per Open Questions).
- The same identity may spectate even as a guest; spectators are never
  bot-backfilled and never counted toward `PlayerCount`.

## 3. Redacted State

The WS server already builds `state_update` **per recipient** via
`stateMessageFor(playerIndex)` (each player gets their own `your_hand`). A
spectator variant `spectatorStateMessage()` reuses the public parts and omits
the private ones:

- **Included**: `board`, `closed_suits`, `ace_close_method`, the `opponents`-style
  per-player public info (display name, avatar, hand **count**, face-down
  **count**, disconnected), `current_turn`, `turn_ends_at`.
- **Excluded**: any `your_hand`, and any `ace_close_options` (hand-derived).
  Face-down card **identities** are never sent mid-game — only counts, exactly
  as opponents see each other today.
- At `game_over`, spectators receive the same `results` payload players do
  (face-down cards are revealed to everyone at that point).

A new message `type: "spectator_state"` (or reuse `state_update` minus the
private fields) carries this. Spectators also receive `player_disconnected` /
`player_reconnected` and `emote` broadcasts so the view stays live.

## 4. Server Design (`services/ws`)

- `room.spectators []*spectator` (connection + identity), guarded by the
  existing `room.mu`.
- Join flow: `handleWebSocket` branches on `role=spectator` → `addSpectator`
  instead of `joinRoom`; sends an immediate `spectator_state` snapshot (or the
  `game_over` payload if the game already finished).
- Broadcast: every place that calls `broadcastState` / `broadcastGameOver` also
  fans the redacted message out to `room.spectators`. A helper
  `broadcastToSpectators(msg)` mirrors `broadcastState`'s lock-snapshot-send
  pattern.
- Spectators are excluded from: `can_start`, ready logic, turn ownership,
  `nextPlayerWithCards`, bot backfill, results, rematch votes, and the lobby
  player list. Their disconnect just removes them from `room.spectators` (no
  grace timer, no DB membership).
- Spectators are **not** persisted in the Redis room snapshot (they're transient
  viewers); on a WS restart they simply reconnect.

## 5. API (`services/api`)

To deep-link a spectatable game, the frontend needs to know a player is in a
live room. Options:

- Extend the leaderboard / profile response with an optional
  `live_room_id` when the player is currently in an in-progress room, derived
  from room membership + status. Requires the API to know live rooms (room
  status is already updated by the WS service via `/internal/rooms/:id/status`).
- Or a dedicated `GET /live-games` listing in-progress public rooms with player
  identities, for a "watch live" browser.

Private rooms are excluded from public discovery; they may still be spectatable
via a direct invite link (see Open Questions).

## 6. Frontend (`web`)

- Route `/watch/:roomId` → a **SpectatorPage** that opens the socket with
  `role=spectator` and renders a read-only board: opponents row with counts, the
  board grid, current-turn indicator and timer — but **no hand, no controls, no
  emote picker**.
- A `useSpectatorSocket` hook (or a `spectator` flag on `useGameSocket`) that
  handles `spectator_state` / `game_over` and never sends moves.
- Entry points: a "Watch" link on leaderboard rows / profiles when the player
  has a `live_room_id`; optionally a live-games list.
- A spectator-count badge shown to seated players (informational).

## 7. Edge Cases

- **No hidden-info leak**: the redaction is server-side; the spectator payload
  never contains a hand. This is the core security property and must be tested.
- **Spectator reconnect**: reconnecting re-sends a fresh snapshot; no seat to
  restore.
- **Game ends / room empties**: spectators receive `game_over`, then the room
  may be cleaned up; the page shows results and a "back" affordance.
- **Spectating a finished room**: serve the `game_over` results snapshot
  (mirrors the existing finished-room reconnect behavior).
- **Private rooms**: not publicly discoverable; direct-link spectating gated by
  an Open Question.
- **Load**: many spectators on one room multiply per-message sends; consider a
  cap.

## 8. Testing

- **Redaction (critical)**: a spectator `state_update` contains no `your_hand`
  and no hidden face-down identities mid-game; counts match.
- Spectator join doesn't change `player_count` / `can_start` and isn't added to
  the lobby player list.
- Spectator receives board updates after each move and the final `game_over`.
- Spectator disconnect removes it without affecting the game; players are
  unaffected.
- Web: SpectatorPage renders board + counts, shows no controls, and never calls
  a send function.
- Run `make -C services/ws test` and `cd web && npm test && npm run lint && npm run build`.

## 9. Rollout

1. WS: spectator role, redacted payload, broadcast fan-out, lifecycle.
2. API: expose `live_room_id` (or `/live-games`) for discovery.
3. Frontend: `/watch/:roomId` page + entry links.
4. No breaking changes to the seated-player protocol.

## 10. Open Questions / Future Work

### Implementation notes (v1 as shipped)

- **Discovery via `GET /live-games`**, not a `live_room_id` field on stats. The
  endpoint lists in-progress public rooms with their seated players; the lobby
  renders a "Watch live" section linking to `/watch/:roomId`.
- **Dedicated `useSpectatorSocket` hook**, not a `spectator` flag on
  `useGameSocket`. It only handles `spectator_state` / `game_over` / `error` and
  has no send function, so a spectator structurally cannot move.
- **Redacted payload** is a distinct `spectator_state` message carrying each
  player's name / avatar / hand **count** / face-down **count** / disconnected —
  verified at the wire to contain no `your_hand` and no `ace_close_options`.
- **`spectator_count`** is included in both `state_update` and `game_over` so
  seated players see how many are watching; spectators join/leave triggers a
  state rebroadcast.
- Spectators are never persisted in the Redis snapshot; a finished/in-progress
  room is rehydrated from the store so a spectator can attach after a restart.

### Still open

- **Private-room spectating** — allow only via an explicit invite/share link, or
  forbid entirely?
- **Lobby spectating** — watch a room before the game starts?
- **Spectator cap** — max concurrent viewers per room to bound fan-out cost.
- **Spectator chat** — a separate, clearly-marked spectator text channel
  (emotes already shipped; see `docs/specs/emotes.md`).
- **Discovery surface** — a full "live games" browser vs. just leaderboard links.
