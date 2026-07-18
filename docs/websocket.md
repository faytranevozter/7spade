# WebSocket Protocol

The WebSocket game server runs at `ws://localhost:8081` (local).

## Connection

### Player

```
ws://localhost:8081/ws?room_id=<room-id>&token=<jwt>
```

### Spectator

```
ws://localhost:8081/ws?room_id=<room-id>&token=<jwt>&role=spectator
```

Unauthenticated or expired tokens cause an immediate connection rejection.

Inbound frames are subject to a **per-connection flood guard** (~40 messages /
10s soft-drop with an error; sustained flood may close the socket). Emotes keep
their own cooldown. Legal `play_card` turns are gated by the turn timer and game
engine, not a low global actions-per-minute cap.

A room moves through two phases:

- **Lobby phase** — players join, mark themselves ready, and the host starts
  the match. Empty seats are filled with bots so the engine always has the
  configured number of hands (default 4, configurable via `max_players`).
- **Playing phase** — turn-based card play until every hand is empty (or a
  stalemate ends the game early).

The server tracks each room in memory (and snapshots it to Redis). Connecting
or reconnecting with the same identity resumes the existing seat.

Spectators join only while a game is `in_progress`. They receive a redacted
`spectator_state` (no hands / ace options), may send cosmetic emotes, and
cannot play cards or vote for rematch.

---

## Message Format

All messages are JSON objects with a `type` field.

**Client → Server**
```json
{ "type": "<event>", "...": "payload" }
```

**Server → Client**
```json
{ "type": "<event>", "...": "payload" }
```

### Message inventory

| Direction | Types |
|---|---|
| Client → Server | `set_ready`, `set_team`, `start_game`, `leave`, `kick`, `emote`, `play_card`, `place_facedown`, `rematch_vote`, `go_to_waiting_room` |
| Server → Client | `lobby_state`, `state_update`, `spectator_state`, `game_over`, `error`, `emote`, `spectator_emote`, `rematch_status`, `rematch_countdown`, `rematch_cancelled`, `player_disconnected`, `player_reconnected`, `room_closed` |

---

## Lobby Phase

On connecting, a player joins the room's lobby. The first player to join is the
**host** (implicitly ready). The server broadcasts a `lobby_state` snapshot
whenever the roster changes.

### Client → Server (lobby)

#### `set_ready`

Toggle your ready flag. Ignored for the host (always ready).

```json
{ "type": "set_ready", "ready": true }
```

#### `set_team`

Choose your team in a 2v2 game. Only valid when the room's `team_mode` is `"2v2"`.
Each team is capped at 2 players. Number of teams = `max_players / 2` (2 for 4p, 3 for 6p).
Must unready before changing team.

```json
{ "type": "set_team", "team": 0 }
```

- `team` is `0` to `(max_players / 2) - 1`.
- Rejected if the target team already has 2 members.
- Rejected if the room is not in 2v2 team mode.

#### `start_game`

Host-only. Starts the match if at least `min_to_start` connected players are
ready (`2` normally; `1` in practice mode). Empty seats are filled with bots,
cards are dealt, and the room enters the playing phase. Players who are
mid-disconnect at start are dropped and backfilled with bots (never dealt in as
phantom humans).

```json
{ "type": "start_game" }
```

#### `leave`

Leave the waiting room immediately. The seat frees up for other players right
away (no reconnect grace). The client sends this before navigating away.

```json
{ "type": "leave" }
```

#### `kick`

Host-only (seat 0), lobby only. Removes another human by seat index and bans
them from rejoining this room (API records the kick). The kicked player receives
`room_closed` with `reason: "kicked"`.

```json
{ "type": "kick", "target": 2 }
```

- `target` is the seat index from `lobby_state.players[].slot`.
- Cannot kick the host (slot 0) or bots.
- Rejected if not host, not lobby, or target not found.

#### `emote` (lobby + playing)

Send a cosmetic reaction. Allowed in lobby and during play. Spectators use the
same client message type (see [Spectator Mode](#spectator-mode)).

```json
{ "type": "emote", "emote": "thumbs_up" }
```

Allowlist: `thumbs_up`, `laugh`, `wow`, `think`, `celebrate`, `sad`, `gg`,
`nice`, `oops`.

- Players: 1s cooldown (faster sends are silently dropped).
- Spectators: 2s cooldown (silently dropped).
- Unknown emote IDs are rejected with `error`.

### Server → Client (lobby)

#### `lobby_state`

Broadcast whenever the roster, ready flags, or connection state change.
Per-recipient: includes `your_slot` for the viewer.

```json
{
  "type": "lobby_state",
  "host_display_name": "Alice",
  "min_to_start": 2,
  "max_players": 4,
  "can_start": true,
  "practice_mode": false,
  "team_mode": "ffa",
  "your_slot": 0,
  "players": [
    {
      "display_name": "Alice",
      "avatar_url": "https://…",
      "slot": 0,
      "is_host": true,
      "ready": true,
      "disconnected": false,
      "team": 0
    },
    {
      "display_name": "Bob",
      "avatar_url": "",
      "slot": 1,
      "is_host": false,
      "ready": true,
      "disconnected": false,
      "team": 1
    }
  ]
}
```

- `your_slot` is the recipient's stable seat index (0-based). Lobby identity
  (host / ready / kick target) should be derived from slot, not display name —
  display names can collide.
- `players[].slot` is the stable seat index (use as `kick.target`).
- `players[].avatar_url` is the player's avatar when set.
- `can_start` is true only when every **connected** player is ready and at least
  `min_to_start` connected players are present.
- A player who drops mid-lobby is still listed with `disconnected: true` during
  the reconnect grace window (~10s) but does **not** count toward `can_start`.
  If they don't reconnect in time, their seat is removed.
- `team_mode` is `"ffa"` or `"2v2"`. When `"2v2"`, each player has a `team` field.
- `max_players` reflects the room's configured player cap (2–8, default 4).
- Practice rooms set `practice_mode: true` and `min_to_start: 1`.

---

## Playing Phase

When the host starts the match, the server deals cards and broadcasts the
initial `state_update` to all room members (and `spectator_state` to spectators).

---

## Client → Server Messages (playing)

### `play_card`

Play a card from your hand. For ranks 2–K this extends a sequence or starts a
new suit with a 7. For an **Ace**, this closes the suit (see
[Closing a suit with an Ace](#closing-a-suit-with-an-ace)).

```json
{ "type": "play_card", "suit": "spades", "rank": "8" }
```

- Only valid on your turn.
- A non-Ace card must be a legal sequence extension or a new-7 start.
- Invalid moves return an error **only to the sender**; state is unchanged.
- Out-of-turn attempts return an error; state is unchanged.

#### Closing a suit with an Ace

Aces never extend a sequence — they only close a suit (low after 2, or high
after K). Send `play_card` with the Ace and an optional `method`:

```json
{ "type": "play_card", "suit": "spades", "rank": "A", "method": "low" }
```

- `method` is `"low"` (Ace = 1 pt) or `"high"` (Ace = 14 pts).
- If omitted, the server infers it from the single legal end, or applies the
  already-locked global close method. If both ends are legal and no method is
  locked yet, the move is rejected as ambiguous — the client must specify one.
- The first close locks the method for the whole game (see
  [Game Rules](./game-rules.md#5-ace-closing-rule-global-consistency)).

### `place_facedown`

Place a card face-down as a penalty.

```json
{ "type": "place_facedown", "suit": "diamonds", "rank": "10" }
```

- Rejected if the player has any valid move, including a closable Ace.

### `rematch_vote`

Vote for a rematch after game over.

```json
{ "type": "rematch_vote" }
```

See [Rematch flow](#rematch-flow).

### `go_to_waiting_room`

Post–game-over only. Resets connected humans back to the lobby phase (new
`lobby_state`). Used when players choose to return to the waiting room instead
of rematching.

```json
{ "type": "go_to_waiting_room" }
```

### `emote`

Same as lobby — see [`emote`](#emote-lobby--playing).

---

## Server → Client Events

### `state_update`

Broadcast after every successful move. Each player receives a version of the
state with **opponent hand contents stripped** (replaced by card counts only),
except teammates in 2v2 who receive each other's full hands.

```json
{
  "type": "state_update",
  "status": "in_progress",
  "board": {
    "spades":   { "low": 5, "high": 9 },
    "hearts":   { "low": 7, "high": 7, "stacks": { "7": 2 } },
    "diamonds": null,
    "clubs":    null
  },
  "closed_suits": ["hearts"],
  "ace_close_method": "high",
  "ace_close_options": [
    { "suit": "spades", "can_low": false, "can_high": true }
  ],
  "your_hand": [
    { "suit": "spades", "rank": "6", "valid": true },
    { "suit": "clubs",  "rank": "J", "valid": false }
  ],
  "your_facedown": [
    { "suit": "diamonds", "rank": "10" }
  ],
  "your_facedown_count": 1,
  "your_index": 0,
  "opponents": [
    {
      "display_name": "Bob",
      "player_index": 1,
      "avatar_url": "https://…",
      "is_bot": false,
      "hand_count": 11,
      "facedown_count": 1,
      "disconnected": false,
      "team": 0,
      "is_teammate": true,
      "hand": [{ "suit": "spades", "rank": "K" }]
    },
    {
      "display_name": "Carol",
      "player_index": 2,
      "avatar_url": "",
      "is_bot": false,
      "hand_count": 12,
      "facedown_count": 0,
      "disconnected": false,
      "team": 1
    },
    {
      "display_name": "Bot 1",
      "player_index": 3,
      "avatar_url": "",
      "is_bot": true,
      "hand_count": 10,
      "facedown_count": 2,
      "disconnected": false,
      "team": 1
    }
  ],
  "current_turn": "Alice",
  "current_turn_index": 0,
  "turn_ends_at": "2024-01-01T10:05:30Z",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "spectator_count": 2,
  "team_info": {
    "team": 0,
    "team_penalty": 5,
    "teammates": ["Bob"]
  }
}
```

- `board` ranges show the current outer edges of each suit's sequence. `null`
  means the suit has not been started yet.
- `board[suit].stacks` (optional, double deck only) maps rank labels to the
  number of cards stacked at that position. Only positions with count > 1 are
  included.
- `closed_suits` lists suits already closed with an Ace.
- `ace_close_method` is the locked global close method (`"low"`, `"high"`, or
  empty until the first close).
- `ace_close_options` lists Aces in your hand that can currently close a suit,
  and which ends are legal — the client uses this to mark the Ace playable and
  to decide whether to prompt for low vs. high.
- A hand card with `valid: true` is a legal play (including a closable Ace).
- `your_facedown` / `your_facedown_count` are your own face-down penalty cards.
- `your_index` is the recipient's stable seat (0-based). `current_turn_index`
  is the seat whose turn it is. Prefer these over display names when matching
  seats (names can collide).
- Each opponent carries `player_index`, `avatar_url`, `is_bot`, and
  `disconnected`.
- In 2v2 mode, opponents include `team` and `is_teammate`. Teammates also include
  `hand` — the full list of cards in their hand (shared hand visibility).
- `team_info` (optional, 2v2 only) shows your team number, the combined team
  penalty so far, and your teammates' display names.
- `bot_difficulty` is one of `easy`, `medium`, or `hard`.
- `practice_mode` is included on `lobby_state`, `state_update`, and `game_over`.
  When `true` the room is a solo-vs-bots practice game: the host can start alone
  (`min_to_start` is `1`), the other seats are bots, and the result is not
  saved to game history or stats.
- `spectator_count` is the number of live spectators.

### `game_over`

Broadcast when all hands are empty (or the game ends early). Also sent when a
player connects to an already-finished room, so the results screen renders
without a prior `state_update`.

```json
{
  "type": "game_over",
  "game_id": "a1b2c3d4-…",
  "board": {
    "spades":   { "low": 2, "high": 13 },
    "hearts":   { "low": 6, "high": 9 },
    "diamonds": null,
    "clubs":    { "low": 7, "high": 7 }
  },
  "closed_suits": ["spades"],
  "ace_close_method": "low",
  "practice_mode": false,
  "team_mode": "ffa",
  "spectator_count": 1,
  "results": [
    {
      "display_name": "Alice",
      "player_index": 0,
      "avatar_url": "https://…",
      "is_bot": false,
      "penalty_points": 5,
      "rank": 1,
      "is_winner": true,
      "facedown_cards": [{ "suit": "clubs", "rank": "5", "points": 5 }],
      "rating_delta": 12,
      "rating_after": 1212,
      "xp_delta": 40,
      "xp_after": 1240,
      "level": 4
    },
    {
      "display_name": "Bob",
      "player_index": 1,
      "avatar_url": "",
      "is_bot": false,
      "penalty_points": 18,
      "rank": 2,
      "is_winner": false,
      "facedown_cards": []
    }
  ]
}
```

- `game_id` is the persisted history id when the game was saved (empty for
  practice / unsaved games).
- `board` / `closed_suits` / `ace_close_method` let the client render the final
  board alongside the results.
- Each result includes `player_index`, `avatar_url`, and `is_bot`.
- `facedown_cards` reveals each player's penalty cards with their point values.
- Tied players both receive `rank: 1` and `is_winner: true`.
- `team_mode` is `"ffa"` or `"2v2"`. In 2v2 mode, each result entry includes a
  `team` field and penalty points reflect the combined team score.
- For registered (non-guest, non-bot) players, when rating/XP deltas were
  computed: `rating_delta`, `rating_after`, `xp_delta`, `xp_after`, `level`.
- `practice_mode` indicates whether this was a practice game (no stats saved).

### `spectator_state`

Redacted live state for spectators only. Same board / turn metadata as players,
but no hands, ace options, or face-down card identities.

```json
{
  "type": "spectator_state",
  "board": {
    "spades": { "low": 5, "high": 9 },
    "hearts": null,
    "diamonds": null,
    "clubs": null
  },
  "closed_suits": [],
  "ace_close_method": "",
  "players": [
    {
      "display_name": "Alice",
      "avatar_url": "https://…",
      "is_bot": false,
      "hand_count": 10,
      "facedown_count": 0,
      "disconnected": false
    }
  ],
  "current_turn": "Alice",
  "turn_ends_at": "2024-01-01T10:05:30Z",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
  "spectator_count": 2
}
```

### `emote`

Broadcast when a seated player sends an emote (including the sender).

```json
{ "type": "emote", "display_name": "Alice", "emote": "gg" }
```

### `spectator_emote`

Broadcast when a spectator sends an emote.

```json
{ "type": "spectator_emote", "spectator_id": "spec-1", "emote": "wow" }
```

### Rematch flow

After `game_over`, connected humans may vote for a rematch. Bots never vote and
are excluded from rematch tallies.

1. First `rematch_vote` opens a countdown window (~30s) and broadcasts
   `rematch_countdown` with `expires_at`.
2. Every vote (including the first) broadcasts `rematch_status`.
3. When **all currently connected humans** have voted → immediate rematch
   (new deal + `state_update`).
4. On timeout:
   - Some voters → voters return to lobby (`lobby_state`); non-voters receive
     `room_closed`.
   - Zero voters → everyone receives `room_closed` and the room is torn down.

#### `rematch_status`

```json
{
  "type": "rematch_status",
  "votes": 2,
  "total": 3,
  "players": [
    { "display_name": "Alice", "player_index": 0, "voted": true,  "left": false },
    { "display_name": "Bob",   "player_index": 1, "voted": true,  "left": false },
    { "display_name": "Carol", "player_index": 2, "voted": false, "left": true }
  ]
}
```

- `total` counts all human seats (including disconnected); bots are omitted.
- `left: true` marks a human who disconnected during the rematch window.

#### `rematch_countdown`

```json
{ "type": "rematch_countdown", "expires_at": "2024-01-01T10:06:00Z" }
```

#### `rematch_cancelled`

Still emitted in some cancel paths; the primary partial-rematch path uses
`room_closed` + lobby return as described above.

```json
{ "type": "rematch_cancelled" }
```

### `room_closed`

The room is no longer available for this client. Optional `reason`:

```json
{ "type": "room_closed" }
```

```json
{ "type": "room_closed", "reason": "kicked" }
```

Sent on host kick, rematch timeout (non-voters / empty room), and room teardown.

### `player_disconnected`

Broadcast when a player's connection drops **during a game**. The auto-play bot
takes over their turns. Also delivered to spectators.

```json
{
  "type": "player_disconnected",
  "display_name": "Dave",
  "player_index": 3
}
```

### `player_reconnected`

Broadcast when a disconnected player reconnects mid-game with a valid JWT.

```json
{
  "type": "player_reconnected",
  "display_name": "Dave",
  "player_index": 3
}
```

### `error`

Sent only to the requesting client on an invalid or out-of-turn action.
May include `fatal: true` for terminal errors.

```json
{ "type": "error", "message": "not your turn" }
```

---

## Spectator Mode

| | |
|---|---|
| Join | `?role=spectator` |
| Inbound | `spectator_state`, `game_over`, `emote`, `spectator_emote`, connection events, `error` |
| Outbound | `emote` only (2s cooldown) |
| Not allowed | play, ready, rematch, kick, leave as player |

See [Spectator Mode spec](./specs/spectator-mode.md).

---

## Turn Timer & Auto-Play

Each turn has a countdown defined by the room's `turn_timer_seconds`
(30 / 60 / 90 / 120 s).

- The `turn_ends_at` timestamp in `state_update` tells the client when the timer
  expires.
- On expiry, the server's **Auto-Play Bot** calls
  `game.PickMoveWithDifficulty(state, playerIndex, difficulty)` and applies the
  move automatically, broadcasting `state_update` as if the player had played
  manually.
- The move is chosen by the room's bot `Strategy` (`easy` / `medium` / `hard`).
  Easy plays the first valid sequence play, then an Ace close, then the first
  card face-down. Medium and hard add board- and opponent-aware heuristics. All
  strategies are deterministic: the same state always produces the same choice.
  See the [Bot Difficulty spec](./specs/bot-difficulty.md) for details.
- The same bot logic fills in for a disconnected player and for the bot-filled
  seats created at game start.
