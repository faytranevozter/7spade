import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { GamePage } from './GamePage'
import { AuthProvider } from '../hooks/AuthProvider'
import { ApiError } from '../api/client'
import { getRoom } from '../api/lobby'
import { useGameSocket, type GameSocketState } from '../hooks/useGameSocket'

vi.mock('../hooks/useGameSocket', () => ({
  useGameSocket: vi.fn(),
}))

vi.mock('../api/lobby', () => ({
  getRoom: vi.fn(),
}))

const sendPlayCard = vi.fn()
const sendFaceDown = vi.fn()
const sendEmote = vi.fn()

const liveState: GameSocketState = {
  status: 'open',
  boardRows: [
    { suit: 'Spades', cards: [null, null, null, null, null, '6', '7', '8', null, null, null, null, null, null] },
    { suit: 'Hearts', cards: [null, null, null, null, null, null, '7', '8', '9', '10', 'J', 'Q', 'K', 'A'], closed: true, aceEnd: 'high' },
    { suit: 'Diamonds', cards: [null, null, null, null, null, null, null, null, null, null, null, null, null, null] },
    { suit: 'Clubs', cards: [null, null, null, null, null, null, '7', '8', '9', null, null, null, null, null] },
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
  turnTimerSeconds: 60,
  rematchVotes: 0,
  rematchTotal: 4,
  gameOver: false,
  results: [],
  emotes: {},
  myDisplayName: 'You',
  sendPlayCard,
  sendFaceDown,
  sendRematchVote: vi.fn(),
  sendSetReady: vi.fn(),
  sendStartGame: vi.fn(),
  sendLeave: vi.fn(),
  sendEmote,
  reconnect: vi.fn(),
}

beforeEach(() => {
	sessionStorage.setItem('seven_spade_auth_token', 'test-token')
	vi.setSystemTime(new Date('2026-05-16T12:00:00Z'))
	vi.mocked(useGameSocket).mockReturnValue(liveState)
	// Default: the room exists, so the existence guard is a no-op.
	vi.mocked(getRoom).mockResolvedValue({
		id: 'room-1',
		invite_code: 'XKQP7A',
		visibility: 'public',
		turn_timer_seconds: 60,
		bot_difficulty: 'medium',
		status: 'in_progress',
		player_count: 4,
	})
})

afterEach(() => {
	cleanup()
	vi.useRealTimers()
	sessionStorage.clear()
	localStorage.clear()
	vi.clearAllMocks()
})

function renderGame() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/game/room-1']}>
        <Routes>
          <Route path="/game/:roomId" element={<GamePage />} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

function renderGameWithRoutes() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/game/room-1']}>
        <Routes>
          <Route path="/game/:roomId" element={<GamePage />} />
          <Route path="/lobby" element={<div>Lobby Landing</div>} />
          <Route path="/history" element={<div>History Landing</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

test('redirects to the lobby when the room does not exist (404)', async () => {
  vi.mocked(getRoom).mockRejectedValue(new ApiError('Room not found', 404))

  renderGameWithRoutes()

  await waitFor(() => {
    expect(screen.getByText('Lobby Landing')).toBeInTheDocument()
  })
  expect(getRoom).toHaveBeenCalledWith('test-token', 'room-1')
})

test('redirects to history when the room is already finished', async () => {
  vi.mocked(getRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    visibility: 'public',
    turn_timer_seconds: 60,
    bot_difficulty: 'medium',
    status: 'finished',
    player_count: 4,
  })

  renderGameWithRoutes()

  await waitFor(() => {
    expect(screen.getByText('History Landing')).toBeInTheDocument()
  })
  expect(screen.queryByRole('region', { name: /Seven Spade game board/i })).not.toBeInTheDocument()
})

test('stays on the game page when the room exists', async () => {
  renderGameWithRoutes()

  await waitFor(() => {
    expect(getRoom).toHaveBeenCalledWith('test-token', 'room-1')
  })
  expect(screen.queryByText('Lobby Landing')).not.toBeInTheDocument()
  expect(screen.queryByText('History Landing')).not.toBeInTheDocument()
  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
})

test('renders live board sequences, closed suits, turn, and opponent counts', () => {
  renderGame()

  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '6 of Spades' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '7 of Spades' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '8 of Spades' })).toBeInTheDocument()
  expect(screen.getByLabelText(/Hearts suit sequence/)).toHaveTextContent('Closed')
  expect(screen.getByText('⚡ Your turn')).toBeInTheDocument()
  expect(screen.getByText('Budi')).toBeInTheDocument()
  expect(screen.getAllByTitle('Cards in hand').length).toBeGreaterThan(0)
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

test('closing an Ace with a single legal end sends that method directly', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    hand: [
      { rank: 'A', suit: 'Hearts', playable: true, aceClose: { canLow: false, canHigh: true } },
    ],
  })

  renderGame()

  fireEvent.click(screen.getByRole('button', { name: 'Play A of Hearts' }))

  // Only the high end is legal, so no prompt — the method is sent immediately.
  expect(sendPlayCard).toHaveBeenCalledWith({ rank: 'A', suit: 'Hearts', playable: true }, 'high')
  expect(screen.queryByRole('dialog', { name: /Close the suit/i })).not.toBeInTheDocument()
})

test('closing an Ace with both ends open prompts for low or high', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    hand: [
      { rank: 'A', suit: 'Hearts', playable: true, aceClose: { canLow: true, canHigh: true } },
    ],
  })

  renderGame()

  fireEvent.click(screen.getByRole('button', { name: 'Play A of Hearts' }))

  // Both ends open: a prompt appears instead of an immediate send.
  expect(sendPlayCard).not.toHaveBeenCalled()
  const dialog = screen.getByRole('dialog', { name: /Close the suit/i })
  expect(dialog).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: /Close low/i }))
  expect(sendPlayCard).toHaveBeenCalledWith({ rank: 'A', suit: 'Hearts', playable: true }, 'low')
})

test('Escape dismisses the close-suit prompt without playing the Ace', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    hand: [
      { rank: 'A', suit: 'Hearts', playable: true, aceClose: { canLow: true, canHigh: true } },
    ],
  })

  renderGame()

  fireEvent.click(screen.getByRole('button', { name: 'Play A of Hearts' }))
  expect(screen.getByRole('dialog', { name: /Close the suit/i })).toBeInTheDocument()

  fireEvent.keyDown(document, { key: 'Escape' })

  expect(screen.queryByRole('dialog', { name: /Close the suit/i })).not.toBeInTheDocument()
  expect(sendPlayCard).not.toHaveBeenCalled()
})

test('renders the closing Ace on the board row without blanking the suit', () => {
  renderGame()

  const heartsRow = screen.getByLabelText(/Hearts suit sequence/)
  // The full 7..K sequence plus the closing Ace are visible; the row is not blank.
  expect(within(heartsRow).getByLabelText('7 of Hearts')).toBeInTheDocument()
  expect(within(heartsRow).getByLabelText('K of Hearts')).toBeInTheDocument()
  expect(within(heartsRow).getByLabelText('A of Hearts')).toBeInTheDocument()
  expect(heartsRow).toHaveTextContent('Closed')
})

test('selects then confirms a face-down card from the hand when no valid moves', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    hand: liveState.hand.map((card) => ({ rank: card.rank, suit: card.suit })),
    isMyTurn: true,
    currentTurnName: 'You',
  })

  renderGame()

  // No separate dialog — selection happens directly in the hand section.
  expect(screen.queryByRole('dialog', { name: /Place a face-down card/i })).not.toBeInTheDocument()

  // Confirm is disabled until a card is selected.
  const confirm = screen.getByRole('button', { name: /Place face-down/i })
  expect(confirm).toBeDisabled()

  fireEvent.click(screen.getByRole('button', { name: /Select A of Hearts for face down/i }))
  expect(sendFaceDown).not.toHaveBeenCalled()

  expect(confirm).toBeEnabled()
  fireEvent.click(confirm)
  expect(sendFaceDown).toHaveBeenCalledWith({ rank: 'A', suit: 'Hearts' })
})

test('shows a countdown timer bar for the active turn', () => {
	renderGame()

	const timer = screen.getByRole('timer', { name: /Turn timer/i })

	expect(timer).toHaveTextContent('00:18')
	expect(screen.getByLabelText(/Turn time remaining/i)).toHaveStyle({ width: '30%' })
})

test('uses configured turn timer duration for countdown progress', () => {
	vi.mocked(useGameSocket).mockReturnValue({
		...liveState,
		turnEndsAt: '2026-05-16T12:01:30Z',
		turnTimerSeconds: 90,
	})

	renderGame()

	expect(screen.getByRole('timer', { name: /Turn timer/i })).toHaveTextContent('01:30')
	expect(screen.getByLabelText(/Turn time remaining/i)).toHaveStyle({ width: '100%' })
})

test('shows disconnected status for disconnected players', () => {
	vi.mocked(useGameSocket).mockReturnValue({
		...liveState,
		players: liveState.players.map((player) => (
			player.name === 'Budi' ? { ...player, disconnected: true } : player
		)),
	})

	renderGame()

	expect(screen.getByText('Disconnected')).toBeInTheDocument()
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

  // The final board is shown read-only alongside the results.
  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Final board/i })).toBeInTheDocument()

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

test('excludes bots from rematch vote status on the results screen', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    gameOver: true,
    rematchVotes: 1,
    rematchTotal: 2,
    results: [
      { player: 'You', rank: 1, penalty: 5, winner: true, faceDownCards: [] },
      { player: 'Budi', rank: 2, penalty: 8, winner: false, faceDownCards: [] },
      { player: 'Bot 1', rank: 3, penalty: 10, winner: false, bot: true, faceDownCards: [] },
      { player: 'Bot 2', rank: 4, penalty: 12, winner: false, bot: true, faceDownCards: [] },
    ],
    players: [
      { name: 'You', initials: 'YU', cardsLeft: 0, faceDownCount: 0, tone: 'green', winner: true, votedRematch: true },
      { name: 'Budi', initials: 'BU', cardsLeft: 0, faceDownCount: 0, tone: 'gold', votedRematch: false },
      { name: 'Bot 1', initials: 'B1', cardsLeft: 0, faceDownCount: 0, tone: 'dark', bot: true, votedRematch: false },
      { name: 'Bot 2', initials: 'B2', cardsLeft: 0, faceDownCount: 0, tone: 'red', bot: true, votedRematch: false },
    ],
  })

  renderGame()

  const voteStatus = screen.getByLabelText('Rematch vote status')
  expect(voteStatus).toHaveTextContent('You')
  expect(voteStatus).toHaveTextContent('Budi')
  expect(voteStatus).not.toHaveTextContent('Bot 1')
  expect(voteStatus).not.toHaveTextContent('Bot 2')
  expect(screen.getByText('1 / 2 voted')).toBeInTheDocument()
  expect(screen.getAllByText('Waiting')).toHaveLength(1)
})

test('opening the emote picker and choosing an emote calls sendEmote', () => {
  renderGame()

  fireEvent.click(screen.getByRole('button', { name: /Open emotes/i }))
  fireEvent.click(screen.getByRole('menuitem', { name: /Thumbs up/i }))

  expect(sendEmote).toHaveBeenCalledWith('thumbs_up')
})

test('renders an emote bubble over an opponent seat from the emotes map', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...liveState,
    emotes: { Budi: { id: 'gg', seq: 1 } },
  })

  renderGame()

  expect(screen.getByRole('status', { name: /Emote: GG/i })).toBeInTheDocument()
})
