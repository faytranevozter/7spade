import { buildBoardRows, detectStateUpdateCues, type SoundState } from '../src/hooks/useGameSocket'

describe('buildBoardRows', () => {
  it('returns four empty suit rows for an empty board', () => {
    const rows = buildBoardRows({})
    expect(rows).toHaveLength(4)
    for (const row of rows) {
      expect(row.cards).toHaveLength(14)
      expect(row.cards.every((c) => c === null)).toBe(true)
      expect(row.closed).toBe(false)
    }
  })

  it('fills the 2..K span by numeric range (col v-1)', () => {
    // hearts 6..8 -> columns 5,6,7 hold 6,7,8.
    const rows = buildBoardRows({ hearts: { low: 6, high: 8 } })
    const hearts = rows.find((r) => r.suit === 'Hearts')!
    expect(hearts.cards[0]).toBeNull() // low-Ace column
    expect(hearts.cards[5]).toBe('6')
    expect(hearts.cards[6]).toBe('7')
    expect(hearts.cards[7]).toBe('8')
    expect(hearts.cards[8]).toBeNull()
    expect(hearts.cards[13]).toBeNull() // high-Ace column
  })

  it('renders the closing Ace in the correct end column', () => {
    const lowClosed = buildBoardRows({ spades: { low: 2, high: 13 } }, ['spades'], 'low')
    const spadesLow = lowClosed.find((r) => r.suit === 'Spades')!
    expect(spadesLow.closed).toBe(true)
    expect(spadesLow.aceEnd).toBe('low')
    expect(spadesLow.cards[0]).toBe('A')
    expect(spadesLow.cards[13]).toBeNull()

    const highClosed = buildBoardRows({ spades: { low: 2, high: 13 } }, ['spades'], 'high')
    const spadesHigh = highClosed.find((r) => r.suit === 'Spades')!
    expect(spadesHigh.cards[0]).toBeNull()
    expect(spadesHigh.cards[13]).toBe('A')
  })

  it('handles face-string ranks in the wire range', () => {
    // diamonds 10..K -> cols 9,10,11,12 (values 10,11,12,13).
    const rows = buildBoardRows({ diamonds: { low: '10', high: 'K' } })
    const diamonds = rows.find((r) => r.suit === 'Diamonds')!
    expect(diamonds.cards[9]).toBe('10')
    expect(diamonds.cards[12]).toBe('K')
  })
})

describe('detectStateUpdateCues', () => {
  const base: SoundState = { boardCardCount: 4, closedSuitCount: 0, handCount: 5, isMyTurn: false }

  it('plays your_turn on the first update when it is my turn', () => {
    expect(detectStateUpdateCues(null, { ...base, isMyTurn: true })).toContain('your_turn')
  })

  it('plays card_play when the board grows', () => {
    const next = { ...base, boardCardCount: 5 }
    expect(detectStateUpdateCues(base, next)).toEqual(['card_play'])
  })

  it('plays card_play when a suit is closed (board count flat)', () => {
    const next = { ...base, closedSuitCount: 1 }
    expect(detectStateUpdateCues(base, next)).toEqual(['card_play'])
  })

  it('plays facedown when the hand shrinks without board growth', () => {
    const next = { ...base, handCount: 4 }
    expect(detectStateUpdateCues(base, next)).toEqual(['facedown'])
  })

  it('adds your_turn when the turn flips to me', () => {
    const next = { ...base, boardCardCount: 5, isMyTurn: true }
    const cues = detectStateUpdateCues(base, next)
    expect(cues).toContain('card_play')
    expect(cues).toContain('your_turn')
  })
})
