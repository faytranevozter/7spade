import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { GamePage } from './GamePage'
import { useGameSocket, type GameSocketState } from '../hooks/useGameSocket'

vi.mock('../hooks/useGameSocket', () => ({
  useGameSocket: vi.fn(),
}))

const sendPlayCard = vi.fn()
const sendFaceDown = vi.fn()

const liveState: GameSocketState = {
  status: 'open',
  boardRows: [
    { suit: 'Spades', cards: [null, null, null, null, null, '6', '7', '8', null, null, null, null, null] },
    { suit: 'Hearts', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null], closed: true },
    { suit: 'Diamonds', cards: [null, null, null, null, null, null, null, null, null, null, null, null, null] },
    { suit: 'Clubs', cards: [null, null, null, null, null, null, '7', '8', '9', null, null, null, null] },
  ],
  hand: [
    { rank: '5', suit: 'Spades' },
    { rank: '9', suit: 'Spades', playable: true },
    { rank: 'A', suit: 'Hearts' },
  ],
  players: [
    { name: 'You', initials: 'YU', cardsLeft: 3, faceDownCount: 0, tone: 'green', active: true },
    { name: 'Budi', initials: 'BU', cardsLeft: 8, faceDownCount: 2, tone: 'gold' },
    { name: 'Santi', initials: 'SA', cardsLeft: 5, faceDownCount: 1, tone: 'dark' },
  ],
  currentTurnName: 'You',
  toasts: [],
  isMyTurn: true,
  turnEndsAt: '2026-05-16T12:00:18Z',
  rematchVotes: 0,
  rematchTotal: 4,
  gameOver: false,
  sendPlayCard,
  sendFaceDown,
  sendRematchVote: vi.fn(),
  reconnect: vi.fn(),
}

beforeEach(() => {
  localStorage.setItem('seven_spade_auth_token', 'test-token')
  vi.mocked(useGameSocket).mockReturnValue(liveState)
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.clearAllMocks()
})

function renderGame() {
  return render(
    <MemoryRouter initialEntries={['/game/room-1']}>
      <Routes>
        <Route path="/game/:roomId" element={<GamePage />} />
      </Routes>
    </MemoryRouter>,
  )
}

test('renders live board sequences, closed suits, turn, and opponent counts', () => {
  renderGame()

  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '6 of Spades' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '7 of Spades' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '8 of Spades' })).toBeInTheDocument()
  expect(screen.getByLabelText('Hearts suit sequence')).toHaveTextContent('Closed')
  expect(screen.getByText('Turn: You')).toBeInTheDocument()
  expect(screen.getByText('Budi')).toBeInTheDocument()
  expect(screen.getByText('8 cards')).toBeInTheDocument()
  expect(screen.getByText('2 face-down')).toBeInTheDocument()
})

test('only valid hand cards are highlighted and clickable on your turn', () => {
  renderGame()

  const playable = screen.getByRole('button', { name: 'Play 9 of Spades' })
  const invalid = screen.getByRole('button', { name: '5 of Spades' })

  expect(playable).toHaveAttribute('data-playable', 'true')
  expect(invalid).toHaveAttribute('data-playable', 'false')
  expect(invalid).toHaveClass('opacity-45')

  fireEvent.click(invalid)
  expect(sendPlayCard).not.toHaveBeenCalled()

  fireEvent.click(playable)
  expect(sendPlayCard).toHaveBeenCalledWith({ rank: '9', suit: 'Spades', playable: true })
})

test('shows face-down selection modal when your turn has no valid moves', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    hand: liveState.hand.map((card) => ({ rank: card.rank, suit: card.suit })),
    isMyTurn: true,
    currentTurnName: 'You',
  })

  renderGame()

  expect(screen.getByRole('dialog', { name: /Place a face-down card/i })).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: /Place A of Hearts face down/i }))

  expect(sendFaceDown).toHaveBeenCalledWith({ rank: 'A', suit: 'Hearts' })
})
