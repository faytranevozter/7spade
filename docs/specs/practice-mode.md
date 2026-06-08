# Spec: Practice Mode (Solo vs Bots)

Status: Implemented
Owner: —
Related: [WebSocket Protocol](../websocket.md) · [HTTP API](../api.md) · [Bot Difficulty](./bot-difficulty.md) · [Roadmap](../roadmap.md)

## 1. Overview

Practice Mode lets a player start a game immediately against three bots, with no
need to wait for other humans. It is aimed at onboarding: a safe space to learn
the rules without affecting history or the leaderboard.

### Goals

- One-tap entry from the lobby into a private solo-vs-bots game.
- Host starts immediately — the usual two-human minimum is bypassed.
- Practice games never appear in the public lobby or live-games list.
- Practice games are never recorded to game history or player stats.
- The host can pick bot difficulty (and turn timer) when starting practice.

### Non-goals

- Saved practice progress, tutorials, or scripted scenarios.
- Per-seat bot difficulty (all three bots share the room difficulty).

## 2. Flow (Option 1: explicit start)

1. Lobby has a prominent **Practice** button (web and mobile) separate from
   *Create room* / *Join by code*.
2. It opens a modal explaining it is solo-vs-bots and not saved, with a bot
   difficulty selector and turn-timer selector.
3. Confirming calls `POST /rooms` with `practice_mode: true`. The API forces the
   room private and persists the flag.
4. The client navigates to the existing waiting room, which shows a **Practice**
   badge, hides invite-sharing controls, and labels the host action
   **Start practice**.
5. The host presses **Start practice**; the WS service fills the three empty
   seats with bots and the game begins. No client-side auto-start is used.

## 3. Data Model

Migration `012_practice_mode.sql` adds the flag to `rooms`:

```sql
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS practice_mode BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_rooms_practice_mode ON rooms(practice_mode);
```

## 4. API

- `POST /rooms` accepts `practice_mode` (optional, default `false`). When `true`,
  `visibility` is forced to `"private"` (it may be omitted by the client).
- Room responses include `practice_mode`.
- `GET /rooms` (public waiting rooms) and `GET /live-games` both exclude rooms
  with `practice_mode = true`, so practice games are never publicly discoverable.

## 5. WebSocket Server

- The WS service loads `practice_mode` from the API room settings on first
  in-memory room creation and stores it on the live room and in the Redis room
  snapshot, so it survives a restart.
- `lobby_state`, `state_update`, and `game_over` payloads include `practice_mode`.
- For practice rooms the lobby start threshold is `1`: `min_to_start` is `1` and
  `can_start` is true once the host is connected and ready. Non-practice rooms
  keep the two-human minimum.
- On game over, the result is **not** sent to the API history endpoint for
  practice rooms (so stats/leaderboard are untouched); the room status is still
  flipped to `finished` for normal cleanup/reconciliation.

## 6. Clients

- Web and mobile lobbies have a Practice button + modal (difficulty + timer).
- The waiting room and the in-game UI show a Practice badge.
- The results screen shows **Practice Mode** and hides the *View history* link
  (practice games are not in history).

## 7. Testing

- API: repository test asserts `CreateRoom` persists `practice_mode` and the
  forced-private visibility.
- WS: tests cover solo start with three bots, the skipped history save, the
  `practice_mode` snapshot round-trip, and that normal rooms still require two
  players.
- Web: lobby test creates a private practice room; waiting-room test shows the
  Practice badge / Start practice button and hides invite sharing; game-over
  test shows Practice Mode and hides the history link.
- Mobile: shares the web socket/board logic; UI verified via typecheck.
