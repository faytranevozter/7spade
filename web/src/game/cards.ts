import type { Suit } from '../types'

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

export const suitColorClass: Record<Suit, string> = {
  Spades: 'text-spade-black',
  Hearts: 'text-spade-red',
  Diamonds: 'text-spade-red',
  Clubs: 'text-spade-black',
}

export const boardSuitColorClass: Record<Suit, string> = {
  Spades: 'text-[#d0cfc9]',
  Hearts: 'text-[#e05c4a]',
  Diamonds: 'text-[#e05c4a]',
  Clubs: 'text-[#d0cfc9]',
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
