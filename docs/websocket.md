# WebSocket Protocol

The WebSocket game server runs at `ws://localhost:8081` (local).

## Connection

Connect to the WS server with a valid JWT:

```
ws://localhost:8081/ws?room_id=<room-id>&token=<jwt>
```

Unauthenticated or expired tokens cause an immediate connection rejection.

When the **4th player** connects to a room, the server automatically deals cards and broadcasts the initial `state_update` to all room members.

---

## Message Format

All messages are JSON objects with a `type` field.

**Client → Server**
```json
{ "type": "<event>", ...payload }
```

**Server → Client**
```json
{ "type": "<event>", ...payload }
```

---

## Client → Server Messages

### `play_card`

Play a card from your hand in sequence.

```json
{
  "type": "play_card",
  "suit": "spades",
  "rank": "8"
}
```

- Only valid on your turn.
- Card must be a legal sequence extension or a new-7 start.
- Invalid moves return an error **only to the sender**; state is unchanged.
- Out-of-turn attempts return an error; state is unchanged.

### `place_facedown`

Place a card face-down as a penalty.

```json
{
  "type": "place_facedown",
  "suit": "diamonds",
  "rank": "10"
}
```

- Rejected if the player has any valid moves (engine enforces this).

### `rematch_vote`

Vote for a rematch after game over.

```json
{ "type": "rematch_vote" }
```

When all 4 players send this, the room resets to `in_progress`, a new deal is performed, and `state_update` is broadcast.

---

## Server → Client Events

### `state_update`

Broadcast after every successful move. Each player receives a version of the state with **opponent hand contents stripped** (replaced by card counts only).

```json
{
  "type": "state_update",
  "board": {
    "spades":   { "low": 5, "high": 9 },
    "hearts":   { "low": 7, "high": 7 },
    "diamonds": null,
    "clubs":    null
  },
  "closed_suits": ["hearts"],
  "ace_close_method": "high",
  "your_hand": [
    { "suit": "spades", "rank": "6", "valid": true },
    { "suit": "clubs",  "rank": "J", "valid": false }
  ],
  "opponents": [
    { "display_name": "Bob",   "hand_count": 11, "facedown_count": 1 },
    { "display_name": "Carol", "hand_count": 12, "facedown_count": 0 },
    { "display_name": "Dave",  "hand_count": 10, "facedown_count": 2 }
  ],
  "current_turn": "Alice",
  "turn_ends_at": "2024-01-01T10:05:30Z"
}
```

`board` ranges show the current outer edges of each suit's sequence. `null` means the suit has not been started yet.

### `game_over`

Broadcast when `IsGameOver` returns true (all hands empty).

```json
{
  "type": "game_over",
  "results": [
    { "display_name": "Alice", "penalty_points": 5,  "rank": 1, "is_winner": true },
    { "display_name": "Bob",   "penalty_points": 5,  "rank": 1, "is_winner": true },
    { "display_name": "Carol", "penalty_points": 18, "rank": 3, "is_winner": false },
    { "display_name": "Dave",  "penalty_points": 22, "rank": 4, "is_winner": false }
  ]
}
```

Tied players both receive `rank: 1` and `is_winner: true`.

### `rematch_status`

Broadcast when any player votes for a rematch, showing current vote tally.

```json
{
  "type": "rematch_status",
  "votes": 2,
  "total": 4,
  "players": [
    { "display_name": "Alice", "voted": true },
    { "display_name": "Bob", "voted": true },
    { "display_name": "Carol", "voted": false },
    { "display_name": "Dave", "voted": false }
  ]
}
```

### `rematch_cancelled`

Broadcast when a player leaves before all 4 have voted for rematch.

```json
{ "type": "rematch_cancelled" }
```

### `player_disconnected`

Broadcast when a player's connection drops during a game. The auto-play bot takes over their turns.

```json
{ "type": "player_disconnected", "display_name": "Dave" }
```

### `player_reconnected`

Broadcast when a disconnected player reconnects with a valid JWT.

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

Each turn has a countdown defined by the room's `turn_timer_seconds` (30 / 60 / 90 / 120 s).

- The `turn_ends_at` timestamp in `state_update` tells the client when the timer expires.
- On expiry, the server's **Auto-Play Bot** calls `PickMove(state, hand)` and applies the move automatically, broadcasting `state_update` as if the player had played manually.
- The bot always picks the first valid move returned by `ValidMoves`, or the first card in hand for face-down placement when no valid moves exist. It is deterministic: same state + hand always produces the same choice.
- The same bot logic applies when a player is disconnected.
