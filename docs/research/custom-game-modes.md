# Custom Game Modes — Research & Impact Analysis

This document maps the current hardcoded assumptions in the Seven Spade codebase and identifies what would need to change to support:

1. Custom penalty points (weighted scoring instead of card-rank-based)
2. Double deck (104 cards instead of 52)
3. Team games (2v2 instead of free-for-all)
4. Other customizations (variable player count, starting suit, etc.)

---

## 1. Game Engine (`services/ws/game/`)

### 1.1 Hardcoded Constants

| Assumption | Location | Line |
|---|---|---|
| Exactly 4 players | `engine.go` | 10 (`const PlayerCount = 4`) |
| 4 suits only | `engine.go` | 14-19 (Spades/Hearts/Diamonds/Clubs) |
| Ranks 2–14 (Ace=14) | `engine.go` | 23-37 |
| 52-card deck | `engine.go` | 92 (`deck := make([]Card, 0, 52)`) |
| Deal distributes cards round-robin to PlayerCount | `engine.go` | 105-107 (`player := index % PlayerCount`) |
| Starter = holder of 7 of Spades | `engine.go` | 108-109 |
| First play must be 7 of Spades | `engine.go` | 348 (`card == (Card{Suit: Spades, Rank: Seven})`) |
| New suits start at 7 | `engine.go` | 350 (`card.Rank == Seven`) |
| Penalty = sum of card rank values | `engine.go` | 254-262 (`CalculateScores`) |
| Ace penalty depends on close method (1/7/14) | `engine.go` | 264-278 (`aceAdjustedValue`) |
| Hands/FaceDown are fixed-size arrays `[PlayerCount][]Card` | `engine.go` | 61-63 |
| Board uses exactly 4 suits | `engine.go` | 85 (`var suits = []Suit{...}`) |
| `fullDeck()` returns 52 cards | `bot.go` | 302-310 |
| Bot logic assumes 52-card universe for inference | `bot.go` | 347-356 (`unknownCards`) |
| `nextPlayerWithCards` wraps modulo PlayerCount | `engine.go` | 376 |

### 1.2 Scoring System

The scoring is determined by `CalculateScores` (engine.go:254) and `aceAdjustedValue` (engine.go:264). Each face-down card contributes its rank value (2–13), and Aces contribute 1, 7, or 14 depending on the global close method.

**For custom penalty points**, you would need:
- A `ScoringConfig` struct (or similar) injected into `GameState` or passed to `CalculateScores`
- Replace `card.PointValue()` / `aceAdjustedValue()` with a configurable mapping (e.g., all face-down cards = 1 point flat, or face cards = 10, etc.)
- The bot scoring helpers (`mediumCardScore`, `hardCardScore`, `lowestPenaltyCard`, `expectedFacedownRisk`) all call `aceAdjustedValue` — they would need the same config. See bot.go:162-163, 219, 262-267, 543.

### 1.3 Double Deck (104 cards)

The deck construction lives in `Deal()` (engine.go:91-118) and `fullDeck()` (bot.go:302-310).

**What changes:**
- `Deal()` must generate 2x each card (or N decks). The 52-capacity hint becomes 104.
- `containsCard` (engine.go:405) returns on first match — with duplicates, `removeCard` already removes only the first occurrence (correct), but `containsCard` is fine too.
- `isPlayable` logic (engine.go:335) doesn't need to change fundamentally (a second 7 of spades is just another 7 that can start a suit — but the suit is already started). However, the concept of "sequence extends by exactly +/-1" means a second copy of a rank already on the board is NOT playable. The engine would need a rule decision: are duplicate cards unplayable (always face-down)? Or does the sequence track multiplicity?
- `GameState.Board` type `map[Suit]SuitSequence` tracks only low/high — it doesn't track card count per rank, so duplicates can't extend the sequence further. This is a fundamental design issue for double deck.
- The bot's `fullDeck()` / `knownCards` / `unknownCards` inference model assumes each card exists exactly once. Double deck breaks this completely.
- `Deal()` with 104 cards / 4 players = 26 cards per hand (vs 13 now). The `[PlayerCount][]Card` slice-of-slices handles variable hand sizes already.

### 1.4 Variable Player Count

`PlayerCount = 4` is a compile-time constant used as:
- Array dimension: `[PlayerCount][]Card` in `GameState.Hands` and `GameState.FaceDown` (engine.go:61-63)
- Loop bounds in `CalculateScores`, `cloneState`, `IsGameOver`, `nextPlayerWithCards`
- Bot logic: `validForPlayer` bounds check (bot.go:60), opponent inference

**To support 2–8 players:**
- Change `Hands`/`FaceDown` from fixed arrays to slices (`[][]Card`)
- Make player count a field of `GameState` (or pass it separately)
- Update `Deal()` to accept player count
- `store.RoomSnapshot.InitialHands` uses `[game.PlayerCount][]game.Card` — must become a slice

---

## 2. WS Service — Room Management (`services/ws/`)

### 2.1 Hardcoded Assumptions

| Assumption | Location | Line |
|---|---|---|
| Bot backfill up to `game.PlayerCount` (4) | `lobby.go` | 562 (`for len(room.players) < game.PlayerCount`) |
| Min 2 players to start | `lobby.go` | 23 (`minPlayersToStart = 2`) |
| Lobby max = `game.PlayerCount` | `lobby.go` | 188 (`len(room.players) >= game.PlayerCount`) |
| Lobby state broadcasts `max_players: game.PlayerCount` | `lobby.go` | 474 |
| Room settings struct has no game-mode fields | `server.go` | 209-213 (`roomSettings`) |
| `roomSnapshot.InitialHands` is `[game.PlayerCount][]game.Card` | `server.go` | 199 |
| Opponent list built as `game.PlayerCount-1` | `server.go` | 2074 |
| `scoredPlayersLocked` uses `game.CalculateScores` directly (no config) | `server.go` | 2454-2455 |
| `scoringValue` duplicates `aceAdjustedValue` logic | `server.go` | 2491-2506 |
| `savedGameResult` has no game-mode metadata | `server.go` | 241-248 |
| Room struct has no `gameMode` / `scoringConfig` / `teamConfig` field | `server.go` | 99-140 |

### 2.2 Room Settings Pipeline

Room settings flow: API creates room (DB) -> WS fetches settings on first join -> applied to room struct.

Current `roomSettings` struct (server.go:209-213):
```go
type roomSettings struct {
    TurnTimerSeconds int    `json:"turn_timer_seconds"`
    BotDifficulty    string `json:"bot_difficulty"`
    PracticeMode     bool   `json:"practice_mode"`
}
```

**To add game mode config**, this struct needs new fields (e.g., `ScoringMode`, `DeckCount`, `TeamMode`, `MaxPlayers`), and the WS must pass them through to the engine's `Deal()` and `CalculateScores()`.

### 2.3 Game Start Flow

`handleStartGame` (lobby.go:519-612):
- Line 559-568: Fills empty seats with bots up to `game.PlayerCount`
- Line 571: Calls `game.Deal(time.Now().UnixNano())` with no config params
- A custom mode would require passing deck/player/scoring config to Deal

### 2.4 Snapshot Persistence

`store.RoomSnapshot` (store/store.go:46-59) and `roomSnapshot` (server.go:187-201) use `[game.PlayerCount][]game.Card` for `InitialHands`. This fixed array must become a slice for variable player counts. The Redis JSON serialization handles slices fine.

---

## 3. API Service (`services/api/`)

### 3.1 Room Creation

`createRoomRequest` (handler/room.go:26-34):
```go
type createRoomRequest struct {
    Name             string `json:"name"`
    Visibility       string `json:"visibility"`
    TurnTimerSeconds int    `json:"turn_timer_seconds"`
    BotDifficulty    string `json:"bot_difficulty"`
    PracticeMode     bool   `json:"practice_mode"`
    MinElo           *int   `json:"min_elo"`
    MaxElo           *int   `json:"max_elo"`
}
```

No fields for scoring mode, deck count, team mode, or max players.

**Validation hardcodes** (handler/room.go:84-85):
```go
var validTurnTimers = map[int]bool{30: true, 60: true, 90: true, 120: true}
var validBotDifficulties = map[string]bool{"easy": true, "medium": true, "hard": true}
```

### 3.2 Database Schema

**`rooms` table** (migrations/002_create_rooms.sql:2-10):
- No `game_mode`, `scoring_mode`, `deck_count`, `team_mode`, or `max_players` columns
- `turn_timer_seconds` is CHECK-constrained to `(30, 60, 90, 120)` — would need updating for new valid values

**`room_players` join logic** (repository/room.go:442):
```go
if currentPlayerCount >= 4 {
    return 0, ErrRoomFull
}
```
Hardcoded to 4. Would need to read a `max_players` column from the room.

**`game_players` table** (migrations/003_game_history.sql:8-16):
- `penalty_points INTEGER` — stores final score, works regardless of scoring formula
- No `team` column

**Quick Play SQL** (repository/room.go:279):
```sql
AND (SELECT COUNT(*) FROM room_players rp WHERE rp.room_id = r.id) < 4
```
Hardcoded player cap of 4.

### 3.3 Required Schema Changes

For custom game modes, you would add a migration:
```sql
ALTER TABLE rooms ADD COLUMN game_mode VARCHAR(20) DEFAULT 'classic';
ALTER TABLE rooms ADD COLUMN max_players INTEGER DEFAULT 4;
ALTER TABLE rooms ADD COLUMN deck_count INTEGER DEFAULT 1;
ALTER TABLE rooms ADD COLUMN scoring_mode VARCHAR(20) DEFAULT 'rank_value';
ALTER TABLE rooms ADD COLUMN team_mode VARCHAR(20) DEFAULT 'ffa';
```

The `rooms.turn_timer_seconds` CHECK constraint (migration 002) would need to be dropped/recreated if new timer values are added.

---

## 4. Frontend — Web (`web/src/`)

### 4.1 Hardcoded Assumptions

| Assumption | Location | Line |
|---|---|---|
| Max 4 seats displayed | `LobbyPage.tsx` | 36 (`dto.player_count >= 4`), 48 (`maxSeats: 4`) |
| Room creation has no game-mode selector | `LobbyPage.tsx` | 264-282, 486-609 |
| Board layout: 4 suits x 14 columns | `game/cards.ts` | 3, 11 |
| Replay logic: `PLAYER_COUNT = 4` | `game/replay.ts` | 6 |
| Opponents array assumed to have 3 entries | `useGameSocket.ts` | 35 (opponents type) |
| WaitingRoom shows `maxPlayers` from server (dynamic) | `WaitingRoomPage.tsx` | 116-118 |
| Room card shows "X / 4 players" | `LobbyPage.tsx` | 36 |
| Game result type has no `team` field | `types.ts` | 61-73 |

### 4.2 What Already Supports Flexibility

- `LobbyState.maxPlayers` is already sent by the server and consumed dynamically (WaitingRoomPage.tsx:116)
- `LobbyState.minToStart` is server-driven (WaitingRoomPage.tsx:112)
- The board rendering iterates over `boardRows` from the server — if a double-deck mode doesn't change suit count, it would still render correctly

### 4.3 Create Room Modal Changes

The create room modal (`LobbyPage.tsx:479-609`) would need new form sections:
- Scoring mode selector (classic / flat-penalty / custom)
- Deck count (1 / 2)
- Team mode (free-for-all / 2v2)
- Max players (2–8)

The API types in `web/src/api/lobby.ts` and `mobile/src/api/lobby.ts` would need matching fields.

---

## 5. Mobile App (`mobile/`)

### 5.1 Relevant Files

| File | Purpose |
|---|---|
| `mobile/src/api/lobby.ts` | `CreateRoomRequest` type — mirrors web, no game-mode fields (line 20-27) |
| `mobile/src/hooks/useGameSocket.ts` | Game socket hook — reuses same reducer logic as web |

### 5.2 Impact

The mobile app reuses the web's pure logic (types, card utilities, useGameSocket reducer). Changes to web types/logic automatically propagate. The UI screens would need new form controls mirroring the web's create-room modal additions.

---

## 6. Team Games (2v2)

### 6.1 Current State: Free-For-All Only

There is no concept of "teams" anywhere in the codebase. Every player competes individually.

### 6.2 Required Changes

**Engine (`services/ws/game/engine.go`):**
- Add a `Teams [][]int` (or `TeamOf [PlayerCount]int`) field to `GameState` or a new `GameConfig`
- `CalculateScores` would need to sum team members' face-down cards into a shared team score
- `IsGameOver` semantics don't change (game ends when all hands empty)
- Ranking/winner logic (`scoredPlayersLocked` in server.go:2454) must rank teams, not individuals

**WS Service:**
- `savedGameResult.Players` / `results()` need a `team` field per player
- The game-over broadcast must communicate team results
- Rematch logic stays the same (human vote, team or individual — teams just affect final display)

**API:**
- `game_players` table needs a `team` column (or a separate `game_teams` table)
- History display must group by team

**Frontend:**
- Results screen must show team totals, not just individual
- Waiting room needs team assignment UI (or auto-assign)

---

## 7. Summary of Key Interfaces That Must Change

### `game.GameState` (engine.go:60-67)
```go
// Current
type GameState struct {
    Hands         [PlayerCount][]Card     // fixed-size array
    Board         map[Suit]SuitSequence
    FaceDown      [PlayerCount][]Card     // fixed-size array
    CurrentPlayer int
    Closed        map[Suit]bool
    CloseMethod   CloseMethod
}

// Proposed direction
type GameState struct {
    Hands         [][]Card               // variable player count
    Board         map[Suit]SuitSequence
    FaceDown      [][]Card               // variable player count
    CurrentPlayer int
    Closed        map[Suit]bool
    CloseMethod   CloseMethod
    Config        *GameConfig            // scoring, deck, teams
}
```

### New `GameConfig` struct (proposed)
```go
type GameConfig struct {
    PlayerCount  int
    DeckCount    int               // 1 or 2
    ScoringMode  string            // "rank_value", "flat", "custom"
    CustomScores map[Rank]int      // optional per-rank penalty override
    TeamMode     string            // "ffa", "2v2"
    Teams        [][]int           // team assignments [[0,2],[1,3]]
    StartingSuit Suit              // which suit's 7 must be played first
}
```

### `roomSettings` (server.go:209-213)
Must be extended to carry game-mode config from the API to the WS engine.

### `createRoomRequest` (handler/room.go:26-34)
Must add game-mode fields validated by the API.

### `rooms` table
Needs new columns for game mode, max players, deck count, scoring mode, team mode.

### `store.RoomSnapshot` (store/store.go:46-59)
`InitialHands` must become a slice. Should include `GameConfig` so a rehydrated room knows its mode.

---

## 8. Recommended Implementation Order

1. **Refactor `PlayerCount` from a constant to a runtime value** — highest impact, touches every layer. Start by converting `[PlayerCount][]Card` to `[][]Card` in `GameState` and propagating.

2. **Introduce `GameConfig`** — a struct that travels with the game state and configures scoring, deck, teams. Wire it through `Deal()`, `CalculateScores()`, and the bot strategies.

3. **Custom penalty scoring** — easiest mode to add once `GameConfig` exists. Only changes `CalculateScores` and bot heuristics.

4. **Double deck** — requires a design decision on how duplicates interact with sequences. Moderate engine changes + bot inference overhaul.

5. **Team mode** — mostly a results/display concern once scoring is configurable. Needs team assignment logic and DB schema.

6. **API + DB schema migration** — add room columns, update validation, extend the settings pipeline.

7. **Frontend** — add mode selectors to the create-room modal, handle new result shapes, support variable player counts in board/lobby UI.
