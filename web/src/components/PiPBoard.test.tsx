import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { PiPBoard } from './PiPBoard'
import type { BoardRow, Card, Player } from '../types'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

const rows: BoardRow[] = [
  { suit: 'Spades', cards: [null, null, null, null, null, null, '7', '8', null, null, null, null, null, null], stacks: { '7': 2 } },
  { suit: 'Hearts', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null, null], closed: true },
]

const hand: Card[] = [
  { rank: '9', suit: 'Spades', playable: true },
  { rank: '5', suit: 'Clubs' },
]

const players: Player[] = [
  { name: 'You', initials: 'YO', cardsLeft: 2, faceDownCount: 0, tone: 'green', active: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 5, faceDownCount: 1, tone: 'gold', isTeammate: true },
]

function renderPiPBoard(overrides: Partial<Parameters<typeof PiPBoard>[0]> = {}) {
  const onPlayCard = vi.fn()
  const onFaceDown = vi.fn()
  render(
    <PiPBoard
      rows={rows}
      isMyTurn
      currentTurnName="You"
      timerLabel="12s"
      timerPercent={50}
      hand={hand}
      faceDownMode={false}
      players={players}
      onPlayCard={onPlayCard}
      onFaceDown={onFaceDown}
      {...overrides}
    />,
  )
  return { onPlayCard, onFaceDown }
}

test('renders compact board state and plays a playable card', () => {
  const { onPlayCard } = renderPiPBoard()

  expect(screen.getByText(/Your turn/)).toBeInTheDocument()
  expect(screen.getByText('12s')).toBeInTheDocument()
  expect(screen.getAllByText('7')).toHaveLength(2)
  expect(screen.getByText('2')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: '9♠' }))
  expect(onPlayCard).toHaveBeenCalledWith(hand[0])
  expect(screen.getByRole('button', { name: '5♣' })).toBeDisabled()
})

test('prompts for ace close method when both ends are legal', () => {
  const ace: Card = { rank: 'A', suit: 'Hearts', playable: true, aceClose: { canLow: true, canHigh: true } }
  const { onPlayCard } = renderPiPBoard({ hand: [ace] })

  fireEvent.click(screen.getByRole('button', { name: 'A♥' }))
  expect(onPlayCard).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole('button', { name: 'High (14)' }))
  expect(onPlayCard).toHaveBeenCalledWith(ace, 'high')
})

test('sends a single legal ace close method directly', () => {
  const ace: Card = { rank: 'A', suit: 'Hearts', playable: true, aceClose: { canLow: false, canHigh: true } }
  const { onPlayCard } = renderPiPBoard({ hand: [ace] })

  fireEvent.click(screen.getByRole('button', { name: 'A♥' }))

  expect(onPlayCard).toHaveBeenCalledWith(ace, 'high')
  expect(screen.queryByRole('button', { name: 'High (14)' })).not.toBeInTheDocument()
})

test('face-down mode selects and confirms a penalty card', () => {
  const { onFaceDown } = renderPiPBoard({ faceDownMode: true })

  expect(screen.getByText(/pick a card to place face-down/i)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '5♣' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm face-down' }))

  expect(onFaceDown).toHaveBeenCalledWith(hand[1])
})

test('renders player summary when it is not my turn', () => {
  renderPiPBoard({ isMyTurn: false, currentTurnName: 'Budi', hand })

  expect(screen.getByText("Budi's turn")).toBeInTheDocument()
  expect(screen.getByText('Budi')).toBeInTheDocument()
  expect(screen.getByText('Teammate')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '9♠' })).not.toBeInTheDocument()
})
