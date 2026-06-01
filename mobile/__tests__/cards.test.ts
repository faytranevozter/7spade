import {
  initialsForName,
  normalizeRank,
  sequenceRankValue,
  suitToWireSuit,
  wireSuitToSuit,
} from '../src/game/cards'

describe('cards', () => {
  describe('normalizeRank', () => {
    it('maps numeric face ranks to letters', () => {
      expect(normalizeRank(11)).toBe('J')
      expect(normalizeRank(12)).toBe('Q')
      expect(normalizeRank(13)).toBe('K')
      expect(normalizeRank(14)).toBe('A')
    })

    it('leaves number ranks as strings', () => {
      expect(normalizeRank(7)).toBe('7')
      expect(normalizeRank('10')).toBe('10')
    })
  })

  describe('sequenceRankValue', () => {
    it('returns numeric value for face strings', () => {
      expect(sequenceRankValue('J')).toBe(11)
      expect(sequenceRankValue('A')).toBe(14)
    })

    it('handles wire numbers (incl. face numbers)', () => {
      expect(sequenceRankValue(7)).toBe(7)
      expect(sequenceRankValue(13)).toBe(13)
    })
  })

  describe('suit conversion round-trips', () => {
    it('maps wire suit to UI suit and back', () => {
      expect(wireSuitToSuit.hearts).toBe('Hearts')
      expect(suitToWireSuit.Hearts).toBe('hearts')
      expect(suitToWireSuit[wireSuitToSuit.spades]).toBe('spades')
    })
  })

  describe('initialsForName', () => {
    it('takes up to two uppercased initials', () => {
      expect(initialsForName('Rini Santi')).toBe('RS')
      expect(initialsForName('budi')).toBe('B')
    })

    it('falls back to ? for empty', () => {
      expect(initialsForName('   ')).toBe('?')
    })
  })
})
