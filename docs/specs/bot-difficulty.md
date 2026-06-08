# Spec: Bot Difficulty Levels

Status: Implemented
Owner: —
Related: [Roadmap](../roadmap.md) · [HTTP API](../api.md) · [WebSocket Protocol](../websocket.md)

## 1. Overview

Rooms can choose how strongly bot seats play. This applies to bot-filled seats
and timer-driven auto-play, with `medium` as the default for bot backfill.

### Goals

- Let hosts choose `easy`, `medium`, or `hard` bot difficulty when creating a room.
- Persist the choice on the room so reconnects and WS restarts keep the same setting.
- Surface the setting in the lobby and waiting-room UI on web and mobile.
- Keep existing clients/rooms on `medium` by default.

### Non-goals

- Machine-learning bot play.
- Per-seat bot difficulty.
- Rating-aware bot behaviour.
- Practice Mode itself is specified separately ([#38](https://github.com/faytranevozter/7spade/issues/38)); the practice-create modal reuses the `bot_difficulty` selector defined here.

## 2. Difficulty Contract

| Difficulty | Behaviour |
|---|---|
| `easy` | Original deterministic behaviour. First valid sequence play, then prefer an Ace close over a face-down penalty, otherwise drop the first hand card. No board analysis, no lookahead. |
| `medium` | Board- and opponent-aware. Scores each playable card by sequence progress minus its penalty value minus how much it opens a suit for opponents, and plays the best. Avoids closing a suit when a safe normal play exists and the close would benefit opponents. When forced face-down, throws the lowest-penalty card. |
| `hard` | Full analysis over the inferred unknown-card universe. Scores plays by retained future flexibility, sequence progress, defensive blocking, and opponent benefit; delays Ace closes that help opponents more than the bot; and discards the lowest expected-risk card face-down (dead cards first, soon-playable cards kept). |

Unknown or empty values are normalised to `medium` inside the WS service. The API
validates room creation and rejects values outside `easy`, `medium`, and `hard`.

## 2a. Strategy Interface

Bot decisions go through a `Strategy` in `services/ws/game/bot.go`:

```go
type Strategy interface {
    ChooseMove(state GameState, playerIndex int) (BotMove, bool)
}

func StrategyFor(difficulty BotDifficulty) Strategy
func PickMoveWithDifficulty(state GameState, playerIndex int, difficulty BotDifficulty) (BotMove, bool)
```

`EasyStrategy`, `MediumStrategy`, and `HardStrategy` implement the interface, and
`StrategyFor` maps a difficulty to its strategy (unknown/empty → medium).

`ChooseMove` takes the player **index**, not a loose hand. This differs
deliberately from the issue's `ChoosePlay(board, hand)` wording: a bare hand has
no link back to seat position or opponent counts, so it cannot support
opponent-aware inference. The index lets a strategy read the bot's own hand
(`state.Hands[playerIndex]`), the board, closed suits, the locked close method,
its own face-down pile, and opponent hand *counts* (`len(state.Hands[i])`).

### No-cheating inference model

The server holds every hand, but strategies must not use hidden opponent cards as
known information. They reason only over:

- The bot's own hand and own face-down pile.
- The board sequences (`state.Board`), closed suits (`state.Closed`), and locked
  `state.CloseMethod`.
- Opponent hand **counts** only — never opponent card values or opponent
  face-down identities.

`knownCards` is the union of the bot's hand, every card on the board, the Ace of
each closed suit (closing consumes the Ace from a hand and sets
`Closed[suit]=true`), and the bot's own face-down cards. `unknownCards` is the
rest of the deck — which legitimately still contains opponents' real cards,
because the bot is not allowed to know them. The `TestUnknownCardsExcludeOnlyPublicAndOwnKnownCards`
test asserts this directly: an opponent's card stays in the unknown universe.

## 3. Data Model

Migration `011_bot_difficulty.sql` adds the setting to `rooms`:

```sql
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS bot_difficulty VARCHAR(10) NOT NULL DEFAULT 'medium';

ALTER TABLE rooms
    ADD CONSTRAINT rooms_bot_difficulty_check
    CHECK (bot_difficulty IN ('easy', 'medium', 'hard'));
```

`medium` is the default so old room-create clients still get a difficulty value.

## 4. API

`POST /rooms` accepts `bot_difficulty`:

```json
{
  "visibility": "public",
  "turn_timer_seconds": 60,
  "bot_difficulty": "medium"
}
```

Room responses include `bot_difficulty` alongside `turn_timer_seconds`, including
`GET /rooms`, `GET /rooms/:id`, and create responses.

## 5. WebSocket Server

The WS service loads `bot_difficulty` from the API's room settings endpoint on
first in-memory room creation. The setting is stored on the live room and in the
Redis room snapshot as `bot_difficulty`, so a restarted WS process rehydrates the
same behaviour.

Bot moves use `game.PickMoveWithDifficulty(state, playerIndex, difficulty)`,
which dispatches to the matching `Strategy`. Timer-driven auto-play for
disconnected humans uses the room difficulty the same way.

## 6. Clients

Web and mobile room creation modals include a bot-difficulty selector. Public
room cards and waiting-room detail badges display the selected difficulty.

## 7. Testing

- Bot unit tests cover strategy dispatch (`StrategyFor`), Easy/Medium/Hard
  play and face-down choices, the no-cheating inference model (`unknownCards`),
  Medium's opponent-friendly-close avoidance, Hard's delayed Ace close, defensive
  blocking, lowest-expected-risk discard, determinism, and invalid-seat guards.
- API tests cover room-create validation and defaulting through existing handler
  coverage.
- WS tests continue to cover bot auto-play and room snapshot rehydration.
- Client type checks verify the new `bot_difficulty` DTO field is threaded
  through room creation and display.
