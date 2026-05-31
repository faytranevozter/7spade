export type Suit = 'Spades' | 'Hearts' | 'Diamonds' | 'Clubs'

export type CloseMethod = 'low' | 'high'

export type Card = {
  rank: string
  suit: Suit
  playable?: boolean
  selected?: boolean
  dimmed?: boolean
  // When set, this card is an Ace that can close its suit. The flags say which
  // ends are currently legal so the UI knows whether to prompt for low/high.
  aceClose?: {
    canLow: boolean
    canHigh: boolean
  }
}

export type Player = {
  name: string
  initials: string
  cardsLeft: number
  faceDownCount: number
  tone: 'green' | 'gold' | 'dark' | 'red'
  active?: boolean
  disconnected?: boolean
  winner?: boolean
  votedRematch?: boolean
}

export type Room = {
  name: string
  code: string
  players: string
  status: string
  timer: string
  open: boolean
  filledSeats: number
  maxSeats: number
  visibility: 'public' | 'private'
}

export type Score = {
  rank: number
  player: string
  cardsLeft: number
  penalty: number
  result: string
  winner?: boolean
  me?: boolean
}

export type RevealedPenaltyCard = Card & {
  points: number
}

export type GameResult = {
  rank: number
  player: string
  penalty: number
  winner: boolean
  faceDownCards: RevealedPenaltyCard[]
}

export type ToastTone = 'success' | 'warn' | 'info' | 'error'

export type Toast = {
  id: number
  tone: ToastTone
  title: string
  body: string
}

export type BoardRow = {
  suit: Suit
  cards: Array<string | null>
  closed?: boolean
  // Which end an Ace closed this suit at, so the board can render the Ace in
  // the 14th column on the correct side ('low' = left, 'high' = right).
  aceEnd?: CloseMethod
}
