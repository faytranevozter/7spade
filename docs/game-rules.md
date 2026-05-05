# Game Rules — Seven Spade

## Objective

End the game with the **lowest total points** from face-down (penalty) cards.

---

## Players

- 4 players
- Standard 52-card deck
- No teams

---

## Card Values (for scoring)

| Card | Point Value |
|---|---|
| 2–10 | Same as number |
| J | 11 |
| Q | 12 |
| K | 13 |
| A | 1 or 14 (depends on how it closes a suit — see [Ace Closing Rule](#ace-closing-rule)) |

---

## Game Flow

### 1. Starting the Game

- Shuffle and deal **13 cards** to each player.
- The player holding **7♠ (Seven of Spades)** must play it face-up to the centre of the table to begin.
- Play proceeds **clockwise** from the player who played 7♠.

### 2. Valid Moves on Your Turn

Each turn, a player must do **one** of the following:

- **Extend an existing sequence** — play a card adjacent (±1) to the current edge of a suit's sequence.  
  _Example: after 7♠ is on the table, valid plays are 6♠ (going down) or 8♠ (going up)._
- **Start a new suit** — play another **7** (7♥, 7♦, or 7♣) to open that suit's sequence.
- **Place a card face-down** — only if no valid sequence extension or new-7 start is possible.

> A player who has a valid card **cannot** voluntarily place face-down. The engine rejects the attempt.

### 3. Face-Down Cards (Penalty)

When a player has no legal play they must place **one card face-down** in front of them. That card's point value becomes a **penalty** counted at game end.

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
- Each player reveals their face-down penalty cards.
- Points from all face-down cards are summed per player.

## Winner

- The player with the **lowest total penalty points** wins.
- In the event of a tie, all tied players **share the win**.

---

## Example Turn

| Turn | Player | Action |
|---|---|---|
| 1 | Player 1 | Plays **7♠** (required opening move) |
| 2 | Player 2 | Plays **6♠** (extends Spades downward) |
| 3 | Player 3 | Has no Spades and no 7s → places **10♦ face-down** (10 penalty points) |
| 4 | Player 4 | Plays **8♠** (extends Spades upward) |
