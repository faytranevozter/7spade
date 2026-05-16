export type Suit = 'Spades' | 'Hearts' | 'Diamonds' | 'Clubs'

export type Card = {
  rank: string
  suit: Suit
  playable?: boolean
  selected?: boolean
  dimmed?: boolean
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
  tone: ToastTone
  title: string
  body: string
}

export type BoardRow = {
  suit: Suit
  cards: Array<string | null>
  closed?: boolean
}
