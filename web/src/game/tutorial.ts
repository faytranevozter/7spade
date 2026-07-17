import type { Card, CloseMethod } from '../types'
import type { WireBoardRange } from '../hooks/useGameSocket'

export const TUTORIAL_STORAGE_KEY = 'seven_spade_tutorial'

export type TutorialStatus = 'completed' | 'skipped'

export type TutorialStepId =
  | 'open_7s'
  | 'extend_sequence'
  | 'new_suit_7'
  | 'ace_close'
  | 'face_down'
  | 'scoring'
  | 'turn_timer'
  | 'summary'

export type TutorialRequiredPlay = {
  rank: string
  suit: Card['suit']
  faceDown?: boolean
}

export type TutorialStep = {
  id: TutorialStepId
  title: string
  body: string
  board: Record<string, WireBoardRange>
  closedSuits?: string[]
  aceCloseMethod?: CloseMethod
  hand: Card[]
  // When set, the player must play one of these cards (or place face-down) to advance.
  // Multiple options are allowed when several legal plays teach the same rule
  // (e.g. 6♠ or 8♠ to extend from 7♠).
  requiredPlay?: TutorialRequiredPlay | TutorialRequiredPlay[]
  // Informational steps advance only via Next.
  isSummary?: boolean
  showTimer?: boolean
  timerSeconds?: number
  faceDownMode?: boolean
  // Simulated score table on the scoring / summary steps.
  scorePreview?: Array<{ name: string; penalty: number; winner?: boolean; me?: boolean }>
}

export function readTutorialStatus(): TutorialStatus | null {
  try {
    const raw = window.localStorage.getItem(TUTORIAL_STORAGE_KEY)
    if (raw === 'completed' || raw === 'skipped') return raw
    return null
  } catch {
    return null
  }
}

export function writeTutorialStatus(status: TutorialStatus): void {
  try {
    window.localStorage.setItem(TUTORIAL_STORAGE_KEY, status)
  } catch {
    // Best-effort (private mode / quota).
  }
}

export function clearTutorialStatus(): void {
  try {
    window.localStorage.removeItem(TUTORIAL_STORAGE_KEY)
  } catch {
    // Best-effort.
  }
}

export function shouldAutoPromptTutorial(): boolean {
  return readTutorialStatus() === null
}

export function isTutorialFinished(): boolean {
  const status = readTutorialStatus()
  return status === 'completed' || status === 'skipped'
}

// Scripted guided-practice scenes. Each step uses a fixed board/hand so the
// player can see legal highlights without a live WS deal. Rule copy matches
// docs/game-rules.md and the live ace-close UX.
export const TUTORIAL_STEPS: TutorialStep[] = [
  {
    id: 'open_7s',
    title: 'Opening move: 7♠',
    body:
      'Every game starts with the Seven of Spades. The player who holds 7♠ must play it face-up to open the board. Play then continues clockwise.',
    board: {},
    hand: [
      { rank: '7', suit: 'Spades', playable: true },
      { rank: '5', suit: 'Hearts', playable: false },
      { rank: '9', suit: 'Clubs', playable: false },
      { rank: 'K', suit: 'Diamonds', playable: false },
    ],
    requiredPlay: { rank: '7', suit: 'Spades' },
  },
  {
    id: 'extend_sequence',
    title: 'Build from 7s outward',
    body:
      'Once a suit is open, extend it by playing a card adjacent to either end. After 7♠, legal plays are 6♠ (low) or 8♠ (high). Build sequences both ways from the 7.',
    board: {
      spades: { low: 7, high: 7 },
    },
    hand: [
      { rank: '6', suit: 'Spades', playable: true },
      { rank: '8', suit: 'Spades', playable: true },
      { rank: '4', suit: 'Hearts', playable: false },
      { rank: 'Q', suit: 'Clubs', playable: false },
    ],
    // Either end is legal — accept 6♠ (low) or 8♠ (high).
    requiredPlay: [
      { rank: '6', suit: 'Spades' },
      { rank: '8', suit: 'Spades' },
    ],
  },
  {
    id: 'new_suit_7',
    title: 'Open a new suit with a 7',
    body:
      'Any other 7 starts a new suit sequence. Here Hearts is still empty — play 7♥ to open it. You cannot start a suit with any other rank.',
    board: {
      spades: { low: 6, high: 8 },
    },
    hand: [
      // 5♠/9♠ are also legal sequence plays (shown as static green highlights);
      // only 7♥ is the guided target (gold pulse).
      { rank: '7', suit: 'Hearts', playable: true },
      { rank: '5', suit: 'Spades', playable: true },
      { rank: '9', suit: 'Spades', playable: true },
      { rank: '3', suit: 'Diamonds', playable: false },
    ],
    requiredPlay: { rank: '7', suit: 'Hearts' },
  },
  {
    id: 'ace_close',
    title: 'Close a suit with an Ace',
    body:
      'Aces never extend a sequence — they only close a suit. Close low after a 2 (Ace worth 1) or high after a King (Ace worth 14). The first Ace close locks that method for every suit this game. When both ends are legal, you choose low or high (same modal as live play).',
    board: {
      spades: { low: 2, high: 13 },
      hearts: { low: 6, high: 8 },
    },
    hand: [
      {
        rank: 'A',
        suit: 'Spades',
        playable: true,
        aceClose: { canLow: true, canHigh: true },
      },
      // Also legal sequence plays (static green); Ace is the guided target (pulse).
      { rank: '5', suit: 'Hearts', playable: true },
      { rank: '9', suit: 'Hearts', playable: true },
      { rank: '4', suit: 'Clubs', playable: false },
    ],
    requiredPlay: { rank: 'A', suit: 'Spades' },
  },
  {
    id: 'face_down',
    title: 'Face-down penalty',
    body:
      'If you have no legal sequence play, new 7, or Ace close, you must place one card face-down. That card becomes a penalty at the end. You cannot place face-down while a legal play exists.',
    board: {
      spades: { low: 2, high: 13 },
      hearts: { low: 5, high: 9 },
      diamonds: { low: 7, high: 10 },
      clubs: { low: 6, high: 8 },
    },
    closedSuits: ['spades'],
    aceCloseMethod: 'high',
    hand: [
      // No legal sequence plays; guided target is 3♦ (pulse). Others stay dimmed.
      { rank: '3', suit: 'Diamonds', playable: false },
      { rank: 'Q', suit: 'Clubs', playable: false },
      { rank: '4', suit: 'Hearts', playable: false },
    ],
    faceDownMode: true,
    requiredPlay: { rank: '3', suit: 'Diamonds', faceDown: true },
  },
  {
    id: 'scoring',
    title: 'Lowest penalty wins',
    body:
      'When hands are empty (or the table is deadlocked), face-down cards are scored. Default scoring uses rank value (2–10 face value, J=11, Q=12, K=13; Ace is 1 or 14 depending on how it closed, or 7 if never closed). Lowest total penalty wins; ties share the win.',
    board: {
      spades: { low: 2, high: 13 },
      hearts: { low: 2, high: 13 },
      diamonds: { low: 7, high: 10 },
      clubs: { low: 6, high: 8 },
    },
    closedSuits: ['spades', 'hearts'],
    aceCloseMethod: 'high',
    hand: [],
    scorePreview: [
      { name: 'You', penalty: 8, winner: true, me: true },
      { name: 'Bot Easy', penalty: 15 },
      { name: 'Bot Medium', penalty: 22 },
      { name: 'Bot Hard', penalty: 31 },
    ],
  },
  {
    id: 'turn_timer',
    title: 'Turn timer & auto-play',
    body:
      'Each turn has a timer (chosen when the room is created). If you do not act in time, the server auto-plays for you — a legal card if one exists, otherwise a face-down penalty. Try playing before the demo timer ends, or wait to watch auto-play.',
    board: {
      spades: { low: 7, high: 9 },
    },
    hand: [
      { rank: '6', suit: 'Spades', playable: true },
      { rank: '10', suit: 'Spades', playable: true },
      { rank: '2', suit: 'Hearts', playable: false },
    ],
    // Either legal play works; if the demo timer hits 0 the tutorial auto-plays 6♠.
    requiredPlay: [
      { rank: '6', suit: 'Spades' },
      { rank: '10', suit: 'Spades' },
    ],
    showTimer: true,
    // Short demo countdown so learners see auto-play without waiting a full minute.
    timerSeconds: 8,
  },
  {
    id: 'summary',
    title: 'You are ready to practice',
    body:
      'Remember: open with 7♠, build suits from 7s outward, close with Aces (method locks), and face-down only when stuck. Lowest penalty wins. Practice mode is solo vs bots and does not affect history, stats, or rating — perfect for trying this out.',
    board: {
      spades: { low: 5, high: 10 },
      hearts: { low: 7, high: 7 },
    },
    hand: [],
    isSummary: true,
  },
]

export function getTutorialStep(index: number): TutorialStep | null {
  if (index < 0 || index >= TUTORIAL_STEPS.length) return null
  return TUTORIAL_STEPS[index]
}

export function normalizeRequiredPlays(
  required: NonNullable<TutorialStep['requiredPlay']>,
): TutorialRequiredPlay[] {
  return Array.isArray(required) ? required : [required]
}

export function cardMatchesRequired(
  card: Card,
  required: NonNullable<TutorialStep['requiredPlay']>,
): boolean {
  return normalizeRequiredPlays(required).some(
    (option) => card.rank === option.rank && card.suit === option.suit,
  )
}

export function isFaceDownRequired(
  required: NonNullable<TutorialStep['requiredPlay']>,
): boolean {
  return normalizeRequiredPlays(required).some((option) => Boolean(option.faceDown))
}

const SUIT_SYMBOL: Record<Card['suit'], string> = {
  Spades: '♠',
  Hearts: '♥',
  Diamonds: '♦',
  Clubs: '♣',
}

export function formatCardShort(rank: string, suit: Card['suit']): string {
  return `${rank}${SUIT_SYMBOL[suit]}`
}

export function isTutorialTarget(
  card: Card,
  required: TutorialStep['requiredPlay'],
): boolean {
  if (!required) return false
  return cardMatchesRequired(card, required)
}

// Explicit click instruction for the guided step (names the target cards).
export function formatTutorialActionHint(step: TutorialStep): string | null {
  if (!step.requiredPlay) return null
  const options = normalizeRequiredPlays(step.requiredPlay)
  const labels = options.map((o) => formatCardShort(o.rank, o.suit))
  const joined =
    labels.length === 1
      ? labels[0]
      : labels.length === 2
        ? `${labels[0]} or ${labels[1]}`
        : `${labels.slice(0, -1).join(', ')}, or ${labels[labels.length - 1]}`

  if (isFaceDownRequired(step.requiredPlay)) {
    return `Select ${joined}, then press Place face down.`
  }
  if (options.some((o) => o.rank === 'A')) {
    return `Click ${joined} to close the suit (you'll pick low or high).`
  }
  if (step.showTimer) {
    return `Click ${joined} before the timer ends — or wait to see auto-play.`
  }
  return `Click ${joined} to continue.`
}

function rankToValue(rank: string): number {
  const faces: Record<string, number> = { J: 11, Q: 12, K: 13, A: 14 }
  return faces[rank] ?? Number(rank)
}

// applyGuidedPlay returns the board/closed state after the player places the
// required card in a guided step (sequence extend, new 7, or Ace close). Face-
// down plays leave the board unchanged. Used so the tutorial board updates
// immediately after a correct play, like a controlled practice hand.
export function applyGuidedPlay(
  step: TutorialStep,
  card: Card,
  opts?: { faceDown?: boolean; aceMethod?: CloseMethod },
): {
  board: Record<string, WireBoardRange>
  closedSuits: string[]
  aceCloseMethod?: CloseMethod
  hand: Card[]
} {
  const wireSuit = card.suit.toLowerCase()
  const hand = step.hand.filter((c) => !(c.rank === card.rank && c.suit === card.suit))
  if (opts?.faceDown || (step.requiredPlay && isFaceDownRequired(step.requiredPlay))) {
    return {
      board: { ...step.board },
      closedSuits: [...(step.closedSuits ?? [])],
      aceCloseMethod: step.aceCloseMethod,
      hand,
    }
  }

  if (card.rank === 'A' && card.aceClose) {
    const method = opts?.aceMethod ?? (card.aceClose.canHigh ? 'high' : 'low')
    return {
      board: { ...step.board },
      closedSuits: [...new Set([...(step.closedSuits ?? []), wireSuit])],
      aceCloseMethod: step.aceCloseMethod ?? method,
      hand,
    }
  }

  const value = rankToValue(card.rank)
  const existing = step.board[wireSuit]
  let nextRange: WireBoardRange
  if (!existing) {
    nextRange = { low: value, high: value }
  } else {
    const low = typeof existing.low === 'number' ? existing.low : rankToValue(String(existing.low))
    const high = typeof existing.high === 'number' ? existing.high : rankToValue(String(existing.high))
    nextRange = { low: Math.min(low, value), high: Math.max(high, value) }
  }

  return {
    board: { ...step.board, [wireSuit]: nextRange },
    closedSuits: [...(step.closedSuits ?? [])],
    aceCloseMethod: step.aceCloseMethod,
    hand,
  }
}
