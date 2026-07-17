import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { TutorialOverlay } from './TutorialOverlay'
import {
  TUTORIAL_STORAGE_KEY,
  TUTORIAL_STEPS,
  normalizeRequiredPlays,
} from '../game/tutorial'

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  cleanup()
  localStorage.clear()
})

test('renders first step and blocks Next until required card is played', () => {
  render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)

  expect(screen.getByTestId('tutorial-overlay')).toBeInTheDocument()
  expect(screen.getByText(/Opening move: 7♠/i)).toBeInTheDocument()
  expect(screen.getByTestId('tutorial-action-hint')).toHaveTextContent(/7♠/)

  const next = screen.getByRole('button', { name: 'Next' })
  expect(next).toBeDisabled()

  // Non-target cards are not playable.
  expect(screen.getByRole('button', { name: '5 of Hearts' })).toHaveAttribute('data-playable', 'false')

  fireEvent.click(screen.getByRole('button', { name: /Play 7 of Spades/i }))
  expect(screen.getByText(/Nice — press Next/i)).toBeInTheDocument()
  expect(next).not.toBeDisabled()
  // Board updates after the guided play (7♠ lands).
  expect(screen.getByRole('button', { name: '7 of Spades' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /Play 7 of Spades/i })).not.toBeInTheDocument()
  expect(screen.queryByTestId('tutorial-action-hint')).not.toBeInTheDocument()
})

test('extend step accepts either highlighted end (6♠ or 8♠)', () => {
  render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)

  fireEvent.click(screen.getByRole('button', { name: /Play 7 of Spades/i }))
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))

  expect(screen.getByText(/Build from 7s outward/i)).toBeInTheDocument()
  expect(screen.getByTestId('tutorial-action-hint')).toHaveTextContent(/6♠/)
  expect(screen.getByTestId('tutorial-action-hint')).toHaveTextContent(/8♠/)
  const next = screen.getByRole('button', { name: 'Next' })
  expect(next).toBeDisabled()

  // 8♠ is highlighted and must advance (not only 6♠).
  fireEvent.click(screen.getByRole('button', { name: /Play 8 of Spades/i }))
  expect(screen.getByText(/Nice — press Next/i)).toBeInTheDocument()
  expect(next).not.toBeDisabled()
  expect(screen.getByRole('button', { name: '8 of Spades' })).toBeInTheDocument()
})

test('new-suit step shows legal cards green, only 7♥ pulses as target', () => {
  render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)

  fireEvent.click(screen.getByRole('button', { name: /Play 7 of Spades/i }))
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  fireEvent.click(screen.getByRole('button', { name: /Play 6 of Spades/i }))
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))

  expect(screen.getByText(/Open a new suit with a 7/i)).toBeInTheDocument()
  expect(screen.getByTestId('tutorial-action-hint')).toHaveTextContent(/7♥/)
  // Target: playable + pulse class
  const target = screen.getByRole('button', { name: /Play 7 of Hearts/i })
  expect(target).toHaveAttribute('data-playable', 'true')
  expect(target.className).toMatch(/anim-tutorial-target/)
  // Other legal plays: green ring, no pulse, not the guided click target
  const legalOther = screen.getByRole('button', { name: /Play 5 of Spades/i })
  expect(legalOther).toHaveAttribute('data-playable', 'true')
  expect(legalOther.className).not.toMatch(/anim-tutorial-target/)
  expect(screen.getByRole('button', { name: '3 of Diamonds' })).toHaveAttribute('data-playable', 'false')
})

function advanceToTurnTimerStep() {
  for (let i = 0; i < TUTORIAL_STEPS.length; i++) {
    const step = TUTORIAL_STEPS[i]
    if (step.id === 'turn_timer') break
    if (step.requiredPlay) {
      const option = normalizeRequiredPlays(step.requiredPlay)[0]
      if (option.faceDown) {
        const playLabel = new RegExp(`Play ${option.rank} of ${option.suit}|${option.rank} of ${option.suit}`, 'i')
        fireEvent.click(screen.getByRole('button', { name: playLabel }))
        fireEvent.click(screen.getByRole('button', { name: /Place face down/i }))
      } else {
        const card = step.hand.find((c) => c.rank === option.rank && c.suit === option.suit)
        fireEvent.click(
          screen.getByRole('button', {
            name: new RegExp(`Play ${option.rank} of ${option.suit}`, 'i'),
          }),
        )
        if (card?.aceClose?.canLow && card.aceClose.canHigh) {
          fireEvent.click(screen.getByRole('button', { name: /Close low/i }))
        }
      }
    }
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  }
}

test('turn-timer step lets the player play before the countdown ends', () => {
  render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)
  advanceToTurnTimerStep()

  expect(screen.getByText(/Turn timer & auto-play/i)).toBeInTheDocument()
  expect(screen.getByTestId('tutorial-timer')).toBeInTheDocument()
  expect(screen.getByTestId('tutorial-action-hint')).toHaveTextContent(/auto-play/i)

  const next = screen.getByRole('button', { name: 'Next' })
  expect(next).toBeDisabled()

  fireEvent.click(screen.getByRole('button', { name: /Play 10 of Spades/i }))
  expect(screen.getByTestId('tutorial-play-status')).toHaveTextContent(/Nice/i)
  expect(next).not.toBeDisabled()
})

test('turn-timer step auto-plays when the demo countdown hits zero', () => {
  vi.useFakeTimers()
  try {
    render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)
    advanceToTurnTimerStep()

    expect(screen.getByText(/Turn timer & auto-play/i)).toBeInTheDocument()
    const next = screen.getByRole('button', { name: 'Next' })
    expect(next).toBeDisabled()

    const seconds = TUTORIAL_STEPS.find((s) => s.id === 'turn_timer')!.timerSeconds ?? 8
    act(() => {
      vi.advanceTimersByTime(seconds * 1000)
    })

    expect(screen.getByTestId('tutorial-play-status')).toHaveTextContent(/auto-played/i)
    expect(next).not.toBeDisabled()
  } finally {
    vi.useRealTimers()
  }
})

test('skip marks tutorial skipped in localStorage and calls onClose', () => {
  const onClose = vi.fn()
  render(<TutorialOverlay onClose={onClose} onStartPractice={vi.fn()} />)

  fireEvent.click(screen.getByRole('button', { name: /Skip tutorial/i }))
  expect(localStorage.getItem(TUTORIAL_STORAGE_KEY)).toBe('skipped')
  expect(onClose).toHaveBeenCalledWith('skipped')
})

test('ace-close step shows low/high prompt when both ends are legal', () => {
  render(<TutorialOverlay onClose={vi.fn()} onStartPractice={vi.fn()} />)

  // Advance through steps that need a required play.
  for (let i = 0; i < 3; i++) {
    const step = TUTORIAL_STEPS[i]
    if (step.requiredPlay) {
      const option = normalizeRequiredPlays(step.requiredPlay)[0]
      const label = option.faceDown
        ? `${option.rank} of ${option.suit}`
        : `Play ${option.rank} of ${option.suit}`
      fireEvent.click(screen.getByRole('button', { name: new RegExp(label, 'i') }))
    }
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  }

  expect(screen.getByText(/Close a suit with an Ace/i)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /Play A of Spades/i }))
  expect(screen.getByTestId('tutorial-ace-close')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /Close high/i }))
  expect(screen.getByText(/Nice — press Next/i)).toBeInTheDocument()
})

test('summary offers Start practice and completes tutorial', () => {
  const onClose = vi.fn()
  const onStartPractice = vi.fn()
  render(<TutorialOverlay onClose={onClose} onStartPractice={onStartPractice} />)

  // Walk all steps: play required cards / face-down, then Next.
  for (let i = 0; i < TUTORIAL_STEPS.length - 1; i++) {
    const step = TUTORIAL_STEPS[i]
    if (!step.requiredPlay) {
      fireEvent.click(screen.getByRole('button', { name: 'Next' }))
      continue
    }
    const option = normalizeRequiredPlays(step.requiredPlay)[0]
    if (option.faceDown) {
      // Target cards use the Play aria-label when marked playable for highlight.
      const playLabel = new RegExp(`Play ${option.rank} of ${option.suit}|${option.rank} of ${option.suit}`, 'i')
      fireEvent.click(screen.getByRole('button', { name: playLabel }))
      fireEvent.click(screen.getByRole('button', { name: /Place face down/i }))
    } else {
      const card = step.hand.find((c) => c.rank === option.rank && c.suit === option.suit)
      fireEvent.click(
        screen.getByRole('button', {
          name: new RegExp(`Play ${option.rank} of ${option.suit}`, 'i'),
        }),
      )
      if (card?.aceClose?.canLow && card.aceClose.canHigh) {
        fireEvent.click(screen.getByRole('button', { name: /Close low/i }))
      }
    }
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  }

  expect(screen.getByText(/You are ready to practice/i)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /Start practice/i }))
  expect(localStorage.getItem(TUTORIAL_STORAGE_KEY)).toBe('completed')
  expect(onClose).toHaveBeenCalledWith('completed')
  expect(onStartPractice).toHaveBeenCalled()
})
