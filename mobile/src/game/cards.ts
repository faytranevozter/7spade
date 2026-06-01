import type { Suit } from '../types'

// Ported from web/src/game/cards.ts. The pure logic (rank/suit conversion,
// board layout, sequence values, initials) is identical to the web app. The
// colour maps return hex values (rather than web Tailwind class strings) since
// React Native renders the card glyphs with direct colour values.

export const suits: Suit[] = ['Spades', 'Hearts', 'Diamonds', 'Clubs']

export const ranks = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K']

// Board column layout. A suit can be closed with an Ace at either end, so the
// board has 14 slots: index 0 is the low-Ace column, index 13 the high-Ace
// column, and indices 1..12 hold ranks 2..K (a sequence rank value r maps to
// column r - 1). Aces never appear in the 2..K span — they only ever close.
export const boardColumns = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A']

// sequenceRankValue maps a wire rank (number or face string) to its numeric
// value (2..14) so board fills can be computed by range.
export function sequenceRankValue(rank: string | number): number {
  const normalized = normalizeRank(rank)
  const faceValues: Record<string, number> = { J: 11, Q: 12, K: 13, A: 14 }
  return faceValues[normalized] ?? Number(normalized)
}

export const suitSymbols: Record<Suit, string> = {
  Spades: '♠',
  Hearts: '♥',
  Diamonds: '♦',
  Clubs: '♣',
}

// Hex colours for a card face on a light card surface.
export const suitColor: Record<Suit, string> = {
  Spades: '#1a1a1a',
  Hearts: '#c0392b',
  Diamonds: '#c0392b',
  Clubs: '#1a1a1a',
}

// Hex colours for a glyph drawn on the dark green board.
export const boardSuitColor: Record<Suit, string> = {
  Spades: '#d0cfc9',
  Hearts: '#e05c4a',
  Diamonds: '#e05c4a',
  Clubs: '#d0cfc9',
}

export const wireSuitToSuit: Record<string, Suit> = {
  spades: 'Spades',
  hearts: 'Hearts',
  diamonds: 'Diamonds',
  clubs: 'Clubs',
}

export const suitToWireSuit: Record<Suit, string> = {
  Spades: 'spades',
  Hearts: 'hearts',
  Diamonds: 'diamonds',
  Clubs: 'clubs',
}

export function normalizeRank(rank: string | number): string {
  const value = String(rank)
  const faceRanks: Record<string, string> = {
    '11': 'J',
    '12': 'Q',
    '13': 'K',
    '14': 'A',
  }

  return faceRanks[value] ?? value
}

export function initialsForName(name: string): string {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('')

  return initials || '?'
}
