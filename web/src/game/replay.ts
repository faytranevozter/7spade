import type { BoardRow } from '../types'
import type { ReplayCardDto, ReplayMoveDto } from '../api/replay'
import { buildBoardRows } from '../hooks/useGameSocket'
import { wireSuitToSuit } from './cards'

const PLAYER_COUNT = 4

// ReplayState is the reconstructed game state at a single point in the move
// sequence. It mirrors the shape that GameBoard/CardFace expect.
export type ReplayState = {
  boardRows: BoardRow[]
  hands: ReplayCardDto[][]
  faceDown: ReplayCardDto[][]
  currentPlayer: number
  closedSuits: string[]
  aceCloseMethod: string
}

type InternalCard = { suit: string; rank: number }
type SuitSequence = { low: number; high: number }

function removeCard(hand: InternalCard[], suit: string, rank: number): InternalCard[] {
  const idx = hand.findIndex((c) => c.suit === suit && c.rank === rank)
  if (idx === -1) return hand
  return [...hand.slice(0, idx), ...hand.slice(idx + 1)]
}

function nextPlayerWithCards(hands: InternalCard[][], current: number): number {
  for (let offset = 1; offset <= PLAYER_COUNT; offset++) {
    const candidate = (current + offset) % PLAYER_COUNT
    if (hands[candidate].length > 0) return candidate
  }
  return current
}

function isGameOver(hands: InternalCard[][]): boolean {
  return hands.every((h) => h.length === 0)
}

function isStalemate(
  hands: InternalCard[][],
  board: Record<string, SuitSequence | null>,
  closed: Set<string>,
): boolean {
  if (isGameOver(hands)) return false
  for (const hand of hands) {
    if (hand.length === 0) continue
    const boardEmpty = Object.values(board).every((v) => v === null)
    for (const card of hand) {
      if (card.rank === 14) continue
      if (closed.has(card.suit)) continue
      const seq = board[card.suit]
      if (!seq) {
        if (boardEmpty && card.suit === 'spades' && card.rank === 7) return false
        if (!boardEmpty && card.rank === 7) return false
        continue
      }
      if (card.rank === seq.low - 1 || card.rank === seq.high + 1) return false
    }
    // check ace closes
    for (const card of hand) {
      if (card.rank !== 14) continue
      if (closed.has(card.suit)) continue
      const seq = board[card.suit]
      if (!seq) continue
      if (seq.low === 2 || seq.high === 13) return false
    }
  }
  return true
}

function finalizeStalemate(hands: InternalCard[][], faceDown: InternalCard[][]): {
  hands: InternalCard[][]
  faceDown: InternalCard[][]
} {
  const newHands = hands.map(() => [] as InternalCard[])
  const newFaceDown = faceDown.map((pile, i) => [...pile, ...hands[i]])
  return { hands: newHands, faceDown: newFaceDown }
}

function applyCardToSequence(
  seq: SuitSequence | null,
  rank: number,
): SuitSequence {
  if (!seq) return { low: rank, high: rank }
  return { low: Math.min(seq.low, rank), high: Math.max(seq.high, rank) }
}

// initialReplayState builds the state at move index -1 (before any move).
export function initialReplayState(initialHands: ReplayCardDto[][]): ReplayState {
  const hands: InternalCard[][] = initialHands.map((hand) =>
    hand.map((c) => ({ suit: c.suit, rank: c.rank })),
  )
  const faceDown: InternalCard[][] = Array.from({ length: PLAYER_COUNT }, () => [])
  const board: Record<string, SuitSequence | null> = {
    spades: null,
    hearts: null,
    diamonds: null,
    clubs: null,
  }

  // find the starter: holder of 7♠
  let currentPlayer = 0
  for (let i = 0; i < hands.length; i++) {
    if (hands[i].some((c) => c.suit === 'spades' && c.rank === 7)) {
      currentPlayer = i
      break
    }
  }

  return toReplayState(hands, faceDown, board, new Set(), '', currentPlayer)
}

function toReplayState(
  hands: InternalCard[][],
  faceDown: InternalCard[][],
  board: Record<string, SuitSequence | null>,
  closed: Set<string>,
  aceCloseMethod: string,
  currentPlayer: number,
): ReplayState {
  const wireBoardRange: Record<string, { low: number | string; high: number | string } | null> = {}
  for (const suit of ['spades', 'hearts', 'diamonds', 'clubs']) {
    const seq = board[suit]
    wireBoardRange[suit] = seq ? { low: seq.low, high: seq.high } : null
  }
  const boardRows = buildBoardRows(wireBoardRange, Array.from(closed), aceCloseMethod)
  return {
    boardRows,
    hands: hands.map((h) => h.map((c) => ({ suit: c.suit, rank: c.rank }))),
    faceDown: faceDown.map((pile) => pile.map((c) => ({ suit: c.suit, rank: c.rank }))),
    currentPlayer,
    closedSuits: Array.from(closed),
    aceCloseMethod,
  }
}

// applyMove applies a single recorded move to the current internal state and
// returns the resulting ReplayState.
function applyMoveToState(
  hands: InternalCard[][],
  faceDown: InternalCard[][],
  board: Record<string, SuitSequence | null>,
  closed: Set<string>,
  aceCloseMethod: string,
  move: ReplayMoveDto,
): ReplayState {
  let newHands = hands.map((h) => [...h])
  let newFaceDown = faceDown.map((pile) => [...pile])
  const newBoard = { ...board }
  const newClosed = new Set(closed)
  let newAceCloseMethod = aceCloseMethod
  let newCurrentPlayer: number

  const { player_index, suit, rank, type, ace_direction } = move

  if (type === 'ace_close') {
    newHands[player_index] = removeCard(newHands[player_index], suit, rank)
    newClosed.add(suit)
    if (!newAceCloseMethod && ace_direction) {
      newAceCloseMethod = ace_direction
    }
    newCurrentPlayer = nextPlayerWithCards(newHands, player_index)
  } else if (type === 'play') {
    newHands[player_index] = removeCard(newHands[player_index], suit, rank)
    newBoard[suit] = applyCardToSequence(newBoard[suit], rank)
    newCurrentPlayer = nextPlayerWithCards(newHands, player_index)
  } else {
    // face_down
    newHands[player_index] = removeCard(newHands[player_index], suit, rank)
    newFaceDown[player_index] = [...newFaceDown[player_index], { suit, rank }]
    newCurrentPlayer = nextPlayerWithCards(newHands, player_index)
  }

  // finalizeIfStalemate
  if (isStalemate(newHands, newBoard, newClosed)) {
    const result = finalizeStalemate(newHands, newFaceDown)
    newHands = result.hands
    newFaceDown = result.faceDown
  }

  return toReplayState(newHands, newFaceDown, newBoard, newClosed, newAceCloseMethod, newCurrentPlayer)
}

// reconstructAt returns the replay state after applying moves[0..index]
// (inclusive). Pass index -1 to get the initial deal state.
export function reconstructAt(
  initialHands: ReplayCardDto[][],
  moves: ReplayMoveDto[],
  index: number,
): ReplayState {
  const hands: InternalCard[][] = initialHands.map((hand) =>
    hand.map((c) => ({ suit: c.suit, rank: c.rank })),
  )
  const faceDown: InternalCard[][] = Array.from({ length: PLAYER_COUNT }, () => [])
  const board: Record<string, SuitSequence | null> = {
    spades: null,
    hearts: null,
    diamonds: null,
    clubs: null,
  }
  const closed = new Set<string>()
  const aceCloseMethod = ''

  // find starter
  let currentPlayer = 0
  for (let i = 0; i < hands.length; i++) {
    if (hands[i].some((c) => c.suit === 'spades' && c.rank === 7)) {
      currentPlayer = i
      break
    }
  }

  let state = toReplayState(hands, faceDown, board, closed, aceCloseMethod, currentPlayer)

  const limit = Math.min(index, moves.length - 1)
  for (let i = 0; i <= limit; i++) {
    // re-derive mutable state from the previous step
    const curHands: InternalCard[][] = state.hands.map((h) => h.map((c) => ({ ...c })))
    const curFaceDown: InternalCard[][] = state.faceDown.map((pile) =>
      pile.map((c) => ({ ...c })),
    )
    const curBoard: Record<string, SuitSequence | null> = {}
    for (const suit of ['spades', 'hearts', 'diamonds', 'clubs']) {
      const seq = state.boardRows.find(
        (r) => r.suit === wireSuitToSuit[suit],
      )
      if (seq) {
        const filled = seq.cards.slice(1, 13).map((v, idx) => ({ rank: idx + 2, present: v !== null }))
        const played = filled.filter((v) => v.present)
        if (played.length > 0) {
          curBoard[suit] = { low: played[0].rank, high: played[played.length - 1].rank }
        } else {
          curBoard[suit] = null
        }
      } else {
        curBoard[suit] = null
      }
    }
    const curClosed = new Set(state.closedSuits)
    state = applyMoveToState(
      curHands,
      curFaceDown,
      curBoard,
      curClosed,
      state.aceCloseMethod,
      moves[i],
    )
  }

  return state
}

// rankLabel converts an engine rank int to a display string (e.g. 14 -> "A").
export function rankLabel(rank: number): string {
  if (rank === 14) return 'A'
  if (rank === 13) return 'K'
  if (rank === 12) return 'Q'
  if (rank === 11) return 'J'
  return String(rank)
}

// suitSymbol converts an engine suit string to a unicode symbol.
export function replaySuitSymbol(suit: string): string {
  const symbols: Record<string, string> = {
    spades: '♠',
    hearts: '♥',
    diamonds: '♦',
    clubs: '♣',
  }
  return symbols[suit] ?? suit
}

// moveLabel returns a human-readable description of a move.
export function moveLabel(move: ReplayMoveDto): string {
  const card = `${rankLabel(move.rank)}${replaySuitSymbol(move.suit)}`
  if (move.type === 'ace_close') {
    return `${card} closes ${move.suit} (${move.ace_direction})`
  }
  if (move.type === 'face_down') {
    return `${card} face down`
  }
  return card
}
