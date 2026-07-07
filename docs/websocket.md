# WebSocket Protocol

The WebSocket game server runs at `ws://localhost:8081` (local).

## Connection

Connect to the WS server with a valid JWT:

```
ws://localhost:8081/ws?room_id=<room-id>&token=<jwt>
```

Unauthenticated or expired tokens cause an immediate connection rejection.

A room moves through two phases:

- **Lobby phase** — players join, mark themselves ready, and the host starts
  the match. Empty seats are filled with bots so the engine always has the
  configured number of hands (default 4, configurable via `max_players`).
- **Playing phase** — turn-based card play until every hand is empty.

The server tracks each room in memory. Connecting (or reconnecting) with the
same identity resumes the existing seat.

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

```json
{ "type": "set_team", "team": 0 }
```

- `team` is `0` to `(max_players / 2) - 1`.
- Rejected if the target team already has 2 members.
- Rejected if the room is not in 2v2 team mode.

#### `start_game`

Host-only. Starts the match if at least `min_to_start` (2) connected players are
ready. Empty seats are filled with bots, cards are dealt, and the room enters
the playing phase. Players who are mid-disconnect at start are dropped and
backfilled with bots (never dealt in as phantom humans).

```json
{ "type": "start_game" }
```

#### `leave`

Leave the waiting room immediately. The seat frees up for other players right
away (no reconnect grace). The client sends this before navigating away.

```json
{ "type": "leave" }
```

### Server → Client (lobby)

#### `lobby_state`

Broadcast whenever the roster, ready flags, or connection state change.

```json
{
  "type": "lobby_state",
  "host_display_name": "Alice",
  "min_to_start": 2,
  "max_players": 4,
  "can_start": true,
  "practice_mode": false,
  "team_mode": "ffa",
  "players": [
    { "display_name": "Alice", "is_host": true,  "ready": true,  "disconnected": false, "team": 0 },
    { "display_name": "Bob",   "is_host": false, "ready": true,  "disconnected": false, "team": 1 }
  ]
}
```

- `can_start` is true only when every **connected** player is ready and at least
  `min_to_start` connected players are present.
- A player who drops mid-lobby is still listed with `disconnected: true` during
  the reconnect grace window (~10s) but does **not** count toward `can_start`.
  If they don't reconnect in time, their seat is removed.
- `team_mode` is `"ffa"` or `"2v2"`. When `"2v2"`, each player has a `team` field (0 or 1).
- `max_players` reflects the room's configured player cap (2–8, default 4).

---

## Playing Phase

When the host starts the match, the server deals cards and broadcasts the
initial `state_update` to all room members.

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

When all connected human players send this, the room resets, a new deal is performed, and
`state_update` is broadcast.

---

## Server → Client Events

### `state_update`

Broadcast after every successful move. Each player receives a version of the
state with **opponent hand contents stripped** (replaced by card counts only).

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
  "opponents": [
    { "display_name": "Bob",   "hand_count": 11, "facedown_count": 1, "disconnected": false, "team": 0, "is_teammate": true, "hand": [{"suit": "spades", "rank": "K"}] },
    { "display_name": "Carol", "hand_count": 12, "facedown_count": 0, "disconnected": false, "team": 1 },
    { "display_name": "Dave",  "hand_count": 10, "facedown_count": 2, "disconnected": true,  "team": 1 }
  ],
  "current_turn": "Alice",
  "turn_ends_at": "2024-01-01T10:05:30Z",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium",
  "practice_mode": false,
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
- Each opponent carries a `disconnected` flag.
- In 2v2 mode, opponents include `team` (0 or 1) and `is_teammate` (true for
  your teammate). Teammates also include `hand` — the full list of cards in
  their hand (shared hand visibility).
- `team_info` (optional, 2v2 only) shows your team number, the combined team
  penalty so far, and your teammates' display names.
- `bot_difficulty` is included on live state payloads and is one of `easy`,
  `medium`, or `hard`; it controls bot seats and timer-driven auto-play.
- `practice_mode` is included on `lobby_state`, `state_update`, and `game_over`.
  When `true` the room is a solo-vs-bots practice game: the host can start alone
  (`min_to_start` is `1`), the other seats are bots, and the result is not
  saved to game history or stats.

### `game_over`

Broadcast when all hands are empty. Also sent on its own when a player connects
to (or reconnects to) an already-finished room, so the results screen renders
without a prior `state_update`.

```json
{
  "type": "game_over",
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
  "results": [
    { "display_name": "Alice", "penalty_points": 5,  "rank": 1, "is_winner": true,
      "facedown_cards": [{ "suit": "clubs", "rank": "5", "points": 5 }] },
    { "display_name": "Bob",   "penalty_points": 5,  "rank": 1, "is_winner": true,  "facedown_cards": [] },
    { "display_name": "Carol", "penalty_points": 18, "rank": 3, "is_winner": false, "facedown_cards": [] },
    { "display_name": "Dave",  "penalty_points": 22, "rank": 4, "is_winner": false, "facedown_cards": [] }
  ]
}
```

- `board`/`closed_suits`/`ace_close_method` let the client render the final
  board alongside the results.
- `facedown_cards` reveals each player's penalty cards with their point values.
- Tied players both receive `rank: 1` and `is_winner: true`.
- `team_mode` is `"ffa"` or `"2v2"`. In 2v2 mode, each result entry includes a
  `team` field (0 or 1) and penalty points reflect the combined team score.
- `practice_mode` indicates whether this was a practice game (no stats saved).

### `rematch_status`

Broadcast when any player votes for a rematch, showing the current tally.

```json
{
  "type": "rematch_status",
  "votes": 2,
  "total": 4,
  "players": [
    { "display_name": "Alice", "voted": true },
    { "display_name": "Bob",   "voted": true },
    { "display_name": "Carol", "voted": false },
    { "display_name": "Dave",  "voted": false }
  ]
}
```

### `rematch_cancelled`

Broadcast when a player disconnects before all 4 have voted for a rematch.

```json
{ "type": "rematch_cancelled" }
```

### `player_disconnected`

Broadcast when a player's connection drops **during a game**. The auto-play bot
takes over their turns.

```json
{ "type": "player_disconnected", "display_name": "Dave" }
```

### `player_reconnected`

Broadcast when a disconnected player reconnects mid-game with a valid JWT.

```json
{ "type": "player_reconnected", "display_name": "Dave" }
```

### `error`

Sent only to the requesting client on an invalid or out-of-turn action.

```json
{ "type": "error", "message": "not your turn" }
```

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
  See the [Bot Difficulty spec](specs/bot-difficulty.md) for details.
- The same bot logic fills in for a disconnected player and for the bot-filled
  seats created at game start.
