# Spec: In-Game Emotes / Quick-Chat

Status: Implemented
Owner: —
Related: [Architecture](../architecture.md) · [WebSocket Protocol](../websocket.md) · [Roadmap](../roadmap.md)

## 1. Overview

A lightweight social layer that lets players send a fixed set of emotes (emoji
reactions plus a few canned phrases) to everyone in their room. Emotes render as
a short-lived bubble over the sender's seat and ride entirely on the existing
WebSocket room-broadcast infrastructure — no new data store, no database, and no
HTTP API changes.

### Goals

- Give players in a 4-player real-time game a quick, expressive way to react.
- Work both in the waiting-room lobby and during active gameplay.
- Keep the surface moderation-free and abuse-resistant.

### Non-goals

- Free-text chat (avoided to sidestep moderation / XSS / content-safety).
- Persisting emotes (they are transient and never written to Redis snapshots).
- Emote history, muting, or per-player emote preferences.
- Direct/private messaging between players.

## 2. Vocabulary

Fixed allowlist shared by client and server. IDs are stable; glyphs/labels are
presentation-only.

| ID | Glyph | Label |
|---|---|---|
| `thumbs_up` | 👍 | Thumbs up |
| `laugh` | 😂 | Laugh |
| `wow` | 😮 | Wow |
| `think` | 🤔 | Thinking |
| `celebrate` | 🎉 | Celebrate |
| `sad` | 😢 | Sad |
| `gg` | GG | GG |
| `nice` | Nice! | Nice |
| `oops` | Oops | Oops |

A fixed vocabulary (not free text) is deliberate: the server validates every
emote against an allowlist, so the broadcast channel can't relay arbitrary
payloads, and there is no profanity / XSS / moderation surface.

## 3. Protocol

One new client→server message type and one new server→client broadcast, both on
the existing game WebSocket (`ws://host/ws?room_id=X&token=JWT`). See
[WebSocket Protocol](../websocket.md).

### Client → server

```json
{ "type": "emote", "emote": "thumbs_up" }
```

`emote` must be an id from the allowlist. The message is accepted in both the
lobby and playing phases, and — unlike a card move — it is **not** gated by turn
ownership, so any player can emote at any time.

### Server → client

```json
{ "type": "emote", "display_name": "Alice", "emote": "thumbs_up" }
```

Broadcast to **every connected human in the room, including the sender**, so all
clients (the sender included) render the bubble from the same message rather
than optimistically. Bots never emote and never receive sends.

### Validation & rate limiting

- Unknown `emote` ids are rejected with the standard error frame
  `{ "type": "error", "message": "unknown emote" }`.
- A per-player cooldown of **1 second** throttles spam. Emotes sent inside the
  cooldown window are **silently dropped** (no error frame), so a rapid sender
  doesn't flood the room or get spammed with error toasts.

## 4. Server Design (`services/ws`)

All changes are in `server.go`, following the existing message-handling and
broadcast patterns.

- **Constant & field.** `messageTypeEmote = "emote"` added to the message-type
  const block; `Emote string` added to the `clientMessage` struct.
- **Allowlist.** Package-level `allowedEmotes map[string]bool` holds the ids from
  §2. `emoteCooldown = time.Second`.
- **Rate-limit state.** `lastEmoteAt time.Time` added to the `player` struct,
  read/written only inside `handleEmote` under `room.mu`.
- **Routing** (`handleMessage`): `messageTypeEmote` is dispatched in the
  lobby-phase `switch` and, in the playing phase, **before** the
  `CurrentPlayer` turn-ownership check (after the rematch-vote check). Both paths
  release `room.mu` before calling `handleEmote`, which manages its own locking.
- **`handleEmote(player, emote)`**: rejects non-allowlisted ids; under `room.mu`,
  drops the emote if within the cooldown, else stamps `lastEmoteAt` and snapshots
  the sender's display name; then calls `broadcastEmote` outside the lock.
- **`broadcastEmote(displayName, emote)`**: mirrors `broadcastPlayerConnection`
  — snapshots connected non-bot players under the lock, sends the payload to each
  outside the lock, **including the sender**.

`SaveGame`, Redis snapshots, the turn timer, and the HTTP API are all untouched;
emotes are transient and never persisted.

## 5. Frontend Design (`web`)

- **Catalog** (`src/game/emotes.ts`): the single client-side source of truth —
  an `emotes: Emote[]` array of `{ id, label, glyph }` plus an `emoteGlyph(id)`
  lookup. The `id`s must match the server allowlist.
- **Socket hook** (`src/hooks/useGameSocket.ts`):
  - `EmoteMessage` added to the inbound message union.
  - `emotes: Record<displayName, ActiveEmote>` state, where
    `ActiveEmote = { id, seq }`. `seq` is a monotonic counter making each arrival
    unique.
  - On an inbound `emote`, `showEmote(displayName, id)` records the latest emote
    for that player and schedules it to clear after `EMOTE_TTL_MS` (4s), mirroring
    the toast-expiry pattern. The clear only fires if a newer emote hasn't
    replaced it (guarded by `seq`).
  - `sendEmote(id)` sends `{ type: 'emote', emote: id }`. `myDisplayName`
    (decoded from the JWT) is exposed so the sender's own seat can render its
    echoed bubble.
  - The emotes map is reset on socket teardown/reconnect.
- **Components**:
  - `EmotePicker` — a floating button that opens a tray of emote buttons; closes
    on select, outside-click, or Escape.
  - `EmoteBubble` — renders a seat's active emote as a bubble. The element is
    keyed on `emote.seq` so React remounts it when the emote changes, re-running
    the mount-only `emote-pop` CSS animation (registered in `index.css`).
- **Surfaces**:
  - `GamePage` — picker floats bottom-right above the toast stack; bubbles render
    over each opponent seat and over the player's own hand.
  - `WaitingRoomPage` — picker in the "Your status" panel; bubbles render over
    each lobby seat avatar.

## 6. Edge Cases

- **Off-turn / lobby**: emotes are intentionally allowed regardless of whose turn
  it is, and before the game starts.
- **Spam**: the 1s server cooldown silently drops rapid emotes; the client's 4s
  TTL keeps at most one bubble per player visible at a time.
- **Repeated/replaced emote**: keying the bubble on `seq` re-triggers the pop
  animation instead of swapping the glyph in place.
- **Reconnect**: emotes are not persisted, so none replay on reconnect; the
  client clears its emote map on socket teardown.
- **Display-name collisions**: the emote map is keyed by display name, matching
  the existing rematch-vote and connect/disconnect keying. Two players sharing a
  display name would share a bubble — consistent with prior art, not corrected
  here.
- **Bots**: never send or receive emotes.

## 7. Testing

- **WS server** (`server_test.go`): a valid emote reaches all clients including
  the sender; an unknown emote returns the error frame; a second emote inside the
  cooldown is dropped; an emote works in the lobby phase. The pre-existing
  unknown-message-type rejection test still holds.
- **Web**: `emotes.ts` catalog shape (unique ids, glyph lookup); the GamePage
  picker calls `sendEmote` with the chosen id and an inbound emote renders a
  bubble over the matching seat. The `useGameSocket` mocks in the page tests
  carry the new `emotes` / `myDisplayName` / `sendEmote` fields.
- Run `make -C services/ws test` and `cd web && npm test && npm run lint && npm run build`.

## 8. Open Questions / Future Work

- **Shared vocabulary source**: the client catalog and server allowlist are
  hand-synced; a generated/shared source or a cross-check test would prevent
  drift (adding an id to only one side makes the picker offer an emote the server
  rejects).
- **Stable player identity**: keying by `user_id` instead of display name would
  remove the collision caveat (shared by rematch/connection features too).
- **Richer reactions**: animated/positional emotes, emote sounds, or a small
  recent-emote log.
