# Game Rules — Seven Spade

## Objective

End the game with the **lowest total points** from face-down (penalty) cards.

---

## Classic setup (default)

- **4 players**, free-for-all (no teams)
- Standard **52-card** deck
- **13 cards** dealt to each player
- Face-down penalties scored by **rank value** (see [Card Values](#card-values-for-scoring))

Custom rooms can change seat count, teams, deck size, and scoring — see
[Custom game modes](#custom-game-modes).

---

## Card Values (for scoring)

Default (`scoring_mode: rank_value`):

| Card | Point Value |
|---|---|
| 2–10 | Same as number |
| J | 11 |
| Q | 12 |
| K | 13 |
| A | 1 or 14 (depends on how it closes a suit — see [Ace Closing Rule](#ace-closing-rule-global-consistency)); **7** if never closed during the game |

---

## Game Flow

### 0. Lobby & Starting a Match

Before any cards are dealt, players gather in a room's waiting lobby:

- A player **creates a room** (public or private, with a chosen turn timer) or
  **joins** an existing one by invite code or from the public list. Quick Play
  and Practice Mode are shortcuts from the lobby UI.
- The **host** (first seated player) is automatically ready. Everyone else marks
  themselves **ready**.
- Once at least **`min_to_start` connected players** are ready (`2` normally;
  `1` in practice mode), the host can **start** the match. Empty seats are
  filled with **bots** up to `max_players` (default 4).
- In **2v2** rooms, players pick a team before ready; each team needs at least
  one human before start; bots fill remaining team seats.
- The host may **kick** another human from the lobby (they cannot rejoin that room).
- A player who leaves the lobby frees their seat immediately; a brief
  disconnect (refresh/network blip) holds the seat for a short grace period so
  they can reconnect.

### 1. Starting the Game

- Shuffle and deal evenly (13 cards each in classic 4p / single deck).
- The player holding **7♠ (Seven of Spades)** must play it face-up to begin
  (with multiple decks, the first 7♠ among seats still starts).
- Play proceeds **clockwise** from the player who played 7♠.

### 2. Valid Moves on Your Turn

Each turn, a player must do **one** of the following:

- **Extend an existing sequence** — play a card adjacent (±1) to the current edge of a suit's sequence.  
  _Example: after 7♠ is on the table, valid plays are 6♠ (going down) or 8♠ (going up)._
- **Start a new suit** — play another **7** (7♥, 7♦, or 7♣) to open that suit's sequence.
- **Close a suit with an Ace** — only when the Ace can legally close low or high (see below). Aces never extend a sequence.
- **Place a card face-down** — only if no valid sequence extension, new-7 start, or Ace close is possible.

> A player who has a valid card **cannot** voluntarily place face-down. The engine rejects the attempt.

### 3. Face-Down Cards (Penalty)

When a player has no legal play they must place **one card face-down** in front of them. That card's point value becomes a **penalty** counted at game end (subject to the room's scoring mode).

### 4. Closing a Suit

A suit is **closed** once an **Ace** is placed on either end of its sequence:

- **Low close** — Ace is placed after 2 (sequence: A–2–3–…–K). The Ace is worth **1 point**.
- **High close** — Ace is placed after K (sequence: 7–8–…–K–A). The Ace is worth **14 points**.

Once a suit is closed, no further cards from that suit may be played.

### 5. Ace Closing Rule (Global Consistency)

The **first Ace close** locks the closing method for the entire game:

- If Spades close **high** (A after K → 14 pts), then Hearts, Diamonds, and Clubs must **also** close high.
- If Spades close **low** (A after 2 → 1 pt), then all remaining suits must also close low.

Attempting to close a suit in the opposite direction after the method is locked is an **illegal move** and will be rejected.

---

## End of Game

- The game ends when **all players run out of cards** (every hand is empty).
- It also ends **early** if the table reaches a dead state — when **no player
  holds a playable card** (no sequence extension, no new 7, and no legal Ace
  close). Because a face-down placement never changes the board, such a state is
  irreversible: every remaining card is destined to become a face-down penalty.
  The game ends immediately and each player's remaining hand cards are scored as
  face-down penalties, rather than grinding through forced face-down turns.
- Each player reveals their face-down penalty cards.
- Points from all face-down cards are summed per player (or per team in 2v2).

## Winner

- The player (or team, in 2v2) with the **lowest total penalty points** wins.
- In the event of a tie, all tied players **share the win**.

---

## Custom game modes

Configured at room creation (`game_mode: custom`). Custom games are **casual**
(no ELO impact) and use the lobby "Custom Game" flow.

| Option | Values | Default | Effect |
|--------|--------|---------|--------|
| `max_players` | 2–8 | 4 | Seats; bots fill empty seats at start |
| `deck_count` | 1 or 2 | 1 | 52 or 104 cards; double deck allows stacked ranks on the board |
| `scoring_mode` | `rank_value`, `flat`, `custom` | `rank_value` | How face-down cards score |
| `custom_scores` | rank 2–14 → points 1–100 | — | Required when scoring is `custom` |
| `team_mode` | `ffa`, `2v2` | `ffa` | 2v2 requires 4 or 6 players; teammates share hand visibility and team score |

**Scoring modes**

- **`rank_value`** — classic table above.
- **`flat`** — 1 point per face-down card.
- **`custom`** — per-rank map supplied at create time.

See [Custom Game Modes](./specs/custom-game-modes.md).

### Practice mode

Solo vs bots: `practice_mode: true`, private room, host can start alone
(`min_to_start = 1`). Results are **not** saved to history, stats, rating, or XP.

---

## Example Turn

| Turn | Player | Action |
|---|---|---|
| 1 | Player 1 | Plays **7♠** (required opening move) |
| 2 | Player 2 | Plays **6♠** (extends Spades downward) |
| 3 | Player 3 | Has no Spades and no 7s → places **10♦ face-down** (10 penalty points) |
| 4 | Player 4 | Plays **8♠** (extends Spades upward) |
