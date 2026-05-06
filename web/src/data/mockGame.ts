import type { Card, Player, Room, Score, Suit, Toast } from '../types'

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

export const rooms: Room[] = [
  { name: 'Meja Santai #1', code: 'XKQP7', players: '3 / 4', status: 'Waiting to start', timer: '60s', open: true },
  { name: 'Pro Room', code: 'PR7A2', players: '1 / 4', status: 'Public room', timer: '30s', open: true },
  { name: 'Friday Night Game', code: 'FNG44', players: '4 / 4', status: 'In progress', timer: '90s', open: false },
]

export const players: Player[] = [
  { name: 'Fahrur', initials: 'FA', cardsLeft: 8, faceDownCount: 2, tone: 'green', active: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 11, faceDownCount: 0, tone: 'gold' },
  { name: 'Santi', initials: 'SA', cardsLeft: 6, faceDownCount: 3, tone: 'dark' },
  { name: 'Rini', initials: 'RI', cardsLeft: 0, faceDownCount: 1, tone: 'red', winner: true },
]

export const reconnectPlayers: Player[] = [
  { name: 'Fahrur', initials: 'FA', cardsLeft: 8, faceDownCount: 2, tone: 'green', active: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 10, faceDownCount: 1, tone: 'gold', disconnected: true },
  { name: 'Santi', initials: 'SA', cardsLeft: 6, faceDownCount: 3, tone: 'dark' },
  { name: 'Rini', initials: 'RI', cardsLeft: 7, faceDownCount: 0, tone: 'red' },
]

export const rematchPlayers: Player[] = [
  { name: 'Fahrur', initials: 'FA', cardsLeft: 0, faceDownCount: 2, tone: 'green', winner: true, votedRematch: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 0, faceDownCount: 5, tone: 'gold', votedRematch: true },
  { name: 'Santi', initials: 'SA', cardsLeft: 0, faceDownCount: 4, tone: 'dark' },
  { name: 'Rini', initials: 'RI', cardsLeft: 0, faceDownCount: 3, tone: 'red', winner: true },
]

export const hand: Card[] = [
  { rank: '2', suit: 'Hearts' },
  { rank: '3', suit: 'Clubs' },
  { rank: '4', suit: 'Hearts' },
  { rank: '5', suit: 'Diamonds' },
  { rank: '6', suit: 'Spades', playable: true },
  { rank: '7', suit: 'Clubs' },
  { rank: '8', suit: 'Diamonds', selected: true },
  { rank: '9', suit: 'Hearts' },
  { rank: '10', suit: 'Spades' },
  { rank: 'J', suit: 'Clubs' },
  { rank: 'Q', suit: 'Diamonds' },
  { rank: 'K', suit: 'Hearts' },
  { rank: 'A', suit: 'Spades' },
]

export const noMoveHand: Card[] = [
  { rank: 'A', suit: 'Spades' },
  { rank: 'J', suit: 'Spades' },
  { rank: 'K', suit: 'Spades' },
  { rank: '3', suit: 'Hearts' },
  { rank: 'J', suit: 'Hearts' },
  { rank: 'A', suit: 'Diamonds' },
  { rank: 'J', suit: 'Diamonds', selected: true },
  { rank: 'A', suit: 'Clubs' },
  { rank: 'J', suit: 'Clubs' },
  { rank: 'Q', suit: 'Clubs' },
  { rank: 'K', suit: 'Clubs' },
]

export const boardRows: Array<{ suit: Suit; cards: Array<string | null>; closed?: boolean }> = [
  { suit: 'Hearts', cards: ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'] },
  { suit: 'Spades', cards: [null, null, null, null, null, '7', '8', null, null, null, null, null, null] },
  { suit: 'Diamonds', cards: [null, null, null, null, null, '7', '8', '9', '10', null, null, null, null] },
  { suit: 'Clubs', cards: [null, null, null, null, null, null, null, null, null, null, null, null, null], closed: true },
]

export const toasts: Toast[] = [
  { tone: 'success', title: 'Card played', body: '8♥ placed on the Hearts row' },
  { tone: 'warn', title: '10 seconds left', body: 'Make your move or prepare a penalty card' },
  { tone: 'info', title: "Budi's turn", body: 'Waiting for another player to move' },
  { tone: 'error', title: 'Invalid move', body: 'That card cannot be played here' },
]

export const scores: Score[] = [
  { rank: 1, player: 'Rini', cardsLeft: 0, penalty: 0, result: 'Winner', winner: true },
  { rank: 2, player: 'Santi', cardsLeft: 6, penalty: 24, result: 'Finished' },
  { rank: 3, player: 'Fahrur (you)', cardsLeft: 8, penalty: 12, result: 'Finished', me: true },
  { rank: 4, player: 'Budi', cardsLeft: 11, penalty: 52, result: 'Finished' },
]

export const history = [
  { room: 'Meja Santai #1', date: 'Today', result: 'Winner', score: '+52', players: 'Fahrur, Budi, Santi, Rini' },
  { room: 'Pro Room', date: 'Yesterday', result: 'Second', score: '+24', players: 'Fahrur, Maya, Dito, Rini' },
  { room: 'Friday Night Game', date: 'Apr 30', result: 'Fourth', score: '-8', players: 'Fahrur, Budi, Santi, Rini' },
]
