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
  results: [],
  sendPlayCard,
  sendFaceDown,
  sendRematchVote: vi.fn(),
  reconnect: vi.fn(),
}

beforeEach(() => {
	localStorage.setItem('seven_spade_auth_token', 'test-token')
	vi.setSystemTime(new Date('2026-05-16T12:00:00Z'))
	vi.mocked(useGameSocket).mockReturnValue(liveState)
})

afterEach(() => {
	cleanup()
	vi.useRealTimers()
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

test('shows a countdown timer bar for the active turn', () => {
	renderGame()

	const timer = screen.getByRole('timer', { name: /Turn timer/i })

	expect(timer).toHaveTextContent('00:18')
	expect(screen.getByLabelText(/Turn time remaining/i)).toHaveStyle({ width: '30%' })
})

test('shows a bot badge for disconnected players', () => {
	vi.mocked(useGameSocket).mockReturnValue({
		...liveState,
		players: liveState.players.map((player) => (
			player.name === 'Budi' ? { ...player, disconnected: true } : player
		)),
	})

	renderGame()

	expect(screen.getByText('Budi').closest('article')).toHaveClass('opacity-55')
	expect(screen.getByText('Bot')).toBeInTheDocument()
})

test('renders game-over scores with revealed penalty cards and shared winners', () => {
  const sendRematchVote = vi.fn()
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    gameOver: true,
    rematchVotes: 0,
    rematchTotal: 4,
    results: [
      {
        player: 'You',
        rank: 1,
        penalty: 5,
        winner: true,
        faceDownCards: [{ rank: '5', suit: 'Clubs', points: 5 }],
      },
      {
        player: 'Budi',
        rank: 1,
        penalty: 5,
        winner: true,
        faceDownCards: [{ rank: 'A', suit: 'Hearts', points: 1 }, { rank: '4', suit: 'Spades', points: 4 }],
      },
      {
        player: 'Santi',
        rank: 3,
        penalty: 12,
        winner: false,
        faceDownCards: [{ rank: 'Q', suit: 'Diamonds', points: 12 }],
      },
    ],
    players: [],
    sendRematchVote,
  })

  renderGame()

  expect(screen.getByRole('table', { name: 'Score table' })).toHaveTextContent('You')
  expect(screen.getByRole('table', { name: 'Score table' })).toHaveTextContent('5')
  expect(screen.getAllByText('Shared winner')).toHaveLength(2)
  expect(screen.getByText('A of Hearts')).toBeInTheDocument()
  expect(screen.getByText('+1')).toBeInTheDocument()
  expect(screen.getByText('Q of Diamonds')).toBeInTheDocument()
  expect(screen.getByText('+12')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: /Vote rematch/i }))
  expect(sendRematchVote).toHaveBeenCalledOnce()
})

test('shows per-player rematch vote status on the results screen', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    gameOver: true,
    rematchVotes: 2,
    rematchTotal: 4,
    results: [
      { player: 'You', rank: 1, penalty: 5, winner: true, faceDownCards: [] },
      { player: 'Budi', rank: 2, penalty: 8, winner: false, faceDownCards: [] },
    ],
    players: [
      { name: 'You', initials: 'YU', cardsLeft: 0, faceDownCount: 0, tone: 'green', winner: true, votedRematch: true },
      { name: 'Budi', initials: 'BU', cardsLeft: 0, faceDownCount: 0, tone: 'gold', votedRematch: false },
      { name: 'Santi', initials: 'SA', cardsLeft: 0, faceDownCount: 0, tone: 'dark', votedRematch: true },
      { name: 'Dave', initials: 'DA', cardsLeft: 0, faceDownCount: 0, tone: 'red', votedRematch: false },
    ],
  })

  renderGame()

  const voteStatus = screen.getByLabelText('Rematch vote status')
  expect(voteStatus).toHaveTextContent('You')
  expect(voteStatus).toHaveTextContent('Budi')
  expect(screen.getByText('2 / 4 voted')).toBeInTheDocument()
  expect(screen.getAllByText('Voted')).toHaveLength(2)
  expect(screen.getAllByText('Waiting')).toHaveLength(2)
})
