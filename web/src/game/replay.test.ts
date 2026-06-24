import { describe, expect, it } from 'vitest'
import { reconstructAt } from './replay'
import type { ReplayCardDto, ReplayMoveDto } from '../api/replay'

// Build a stripped-down deal: each seat gets a few specific cards. The actual
// distribution doesn't matter for these tests — only that the move log applies
// cleanly to the hands we hand it.
function deal(hands: ReplayCardDto[][]): ReplayCardDto[][] {
  return hands
}

describe('reconstructAt', () => {
  it('returns initial state at index -1 with no moves applied', () => {
    const hands = deal([
      [{ suit: 'spades', rank: 7 }, { suit: 'hearts', rank: 8 }],
      [{ suit: 'hearts', rank: 7 }],
      [{ suit: 'diamonds', rank: 7 }],
      [{ suit: 'clubs', rank: 7 }],
    ])
    const state = reconstructAt(hands, [], -1)
    expect(state.hands[0]).toHaveLength(2)
    expect(state.hands[1]).toHaveLength(1)
    expect(state.currentPlayer).toBe(0) // seat 0 holds 7♠
    expect(state.closedSuits).toEqual([])
    // No moves: board is empty (all rows have only null slots)
    for (const row of state.boardRows) {
      expect(row.cards.every((c) => c === null)).toBe(true)
    }
  })

  it('applies a single play: removes from hand, extends board', () => {
    const hands = deal([
      [{ suit: 'spades', rank: 7 }],
      [],
      [],
      [],
    ])
    const moves: ReplayMoveDto[] = [
      { index: 0, player_index: 0, suit: 'spades', rank: 7, type: 'play' },
    ]
    const state = reconstructAt(hands, moves, 0)
    expect(state.hands[0]).toHaveLength(0)
    // The 7♠ slot (column 6 in the 14-slot board: A,2..K,A → index 6) is filled.
    const spadesRow = state.boardRows.find((r) => r.suit === 'Spades')!
    expect(spadesRow.cards[6]).toBe('7')
  })

  it('ace_close marks suit closed and records direction', () => {
    // Seat 0 needs the Ace of spades, plus a played sequence reaching King.
    // Simplest setup: 0 plays 7♠ then closes spades high.
    const hands = deal([
      [
        { suit: 'spades', rank: 7 },
        { suit: 'spades', rank: 8 },
        { suit: 'spades', rank: 9 },
        { suit: 'spades', rank: 10 },
        { suit: 'spades', rank: 11 },
        { suit: 'spades', rank: 12 },
        { suit: 'spades', rank: 13 },
        { suit: 'spades', rank: 14 }, // Ace
      ],
      [],
      [],
      [],
    ])
    const moves: ReplayMoveDto[] = [
      { index: 0, player_index: 0, suit: 'spades', rank: 7, type: 'play' },
      { index: 1, player_index: 0, suit: 'spades', rank: 8, type: 'play' },
      { index: 2, player_index: 0, suit: 'spades', rank: 9, type: 'play' },
      { index: 3, player_index: 0, suit: 'spades', rank: 10, type: 'play' },
      { index: 4, player_index: 0, suit: 'spades', rank: 11, type: 'play' },
      { index: 5, player_index: 0, suit: 'spades', rank: 12, type: 'play' },
      { index: 6, player_index: 0, suit: 'spades', rank: 13, type: 'play' },
      { index: 7, player_index: 0, suit: 'spades', rank: 14, type: 'ace_close', ace_direction: 'high' },
    ]
    const state = reconstructAt(hands, moves, 7)
    expect(state.closedSuits).toContain('spades')
    expect(state.aceCloseMethod).toBe('high')
    const spadesRow = state.boardRows.find((r) => r.suit === 'Spades')!
    // High Ace slot is column 13
    expect(spadesRow.cards[13]).toBe('A')
    expect(spadesRow.closed).toBe(true)
  })

  it('face_down moves card from hand to that seat\'s face-down pile', () => {
    // Set up a non-stalemate scenario: seats 2 and 3 still have playable 7s
    // after move 1, so the engine doesn't sweep remaining hands.
    const hands = deal([
      [{ suit: 'spades', rank: 7 }],
      [{ suit: 'clubs', rank: 14 }], // dead Ace (clubs has no sequence yet)
      [{ suit: 'diamonds', rank: 7 }],
      [{ suit: 'hearts', rank: 7 }],
    ])
    const moves: ReplayMoveDto[] = [
      { index: 0, player_index: 0, suit: 'spades', rank: 7, type: 'play' },
      { index: 1, player_index: 1, suit: 'clubs', rank: 14, type: 'face_down' },
    ]
    const state = reconstructAt(hands, moves, 1)
    expect(state.hands[1]).toHaveLength(0)
    expect(state.faceDown[1]).toEqual([{ suit: 'clubs', rank: 14 }])
  })

  it('stepping backwards reconstructs the same state', () => {
    const hands = deal([
      [{ suit: 'spades', rank: 7 }, { suit: 'spades', rank: 8 }],
      [],
      [],
      [],
    ])
    const moves: ReplayMoveDto[] = [
      { index: 0, player_index: 0, suit: 'spades', rank: 7, type: 'play' },
      { index: 1, player_index: 0, suit: 'spades', rank: 8, type: 'play' },
    ]
    const after0a = reconstructAt(hands, moves, 0)
    const after0b = reconstructAt(hands, moves, 0)
    expect(after0a.hands[0]).toEqual(after0b.hands[0])
    expect(after0a.boardRows).toEqual(after0b.boardRows)
  })
})
