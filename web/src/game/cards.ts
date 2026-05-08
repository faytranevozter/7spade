import type { Suit } from '../types'

export const suits: Suit[] = ['Spades', 'Hearts', 'Diamonds', 'Clubs']

export const ranks = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K']

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
