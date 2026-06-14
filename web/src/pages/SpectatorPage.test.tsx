import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { SpectatorPage } from './SpectatorPage'
import { AuthProvider } from '../hooks/AuthProvider'
import { ActiveRoomProvider } from '../hooks/ActiveRoomProvider'
import { useSpectatorSocket, type SpectatorState } from '../hooks/useSpectatorSocket'
import { buildBoardRows } from '../hooks/useGameSocket'
import { getMyActiveRoom } from '../api/lobby'

vi.mock('../hooks/useSpectatorSocket', () => ({
  useSpectatorSocket: vi.fn(),
}))

vi.mock('../api/lobby', () => ({
  getMyActiveRoom: vi.fn().mockResolvedValue({ active_room: null }),
}))

const liveState: SpectatorState = {
  status: 'open',
  notFound: false,
  gameOver: false,
  boardRows: buildBoardRows({ spades: { low: 6, high: 8 } }),
  players: [
    { displayName: 'Alice', handCount: 11, faceDownCount: 1, disconnected: false },
    { displayName: 'Bob', handCount: 13, faceDownCount: 0, disconnected: false },
  ],
  currentTurnName: 'Alice',
  turnEndsAt: null,
  results: [],
  reconnect: vi.fn(),
}

beforeEach(() => {
  sessionStorage.setItem('seven_spade_auth_token', 'test-token')
  vi.mocked(useSpectatorSocket).mockReturnValue(liveState)
})

afterEach(() => {
  cleanup()
  sessionStorage.clear()
  vi.clearAllMocks()
})

function renderSpectator() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/watch/room-1']}>
        <Routes>
          <Route path="/watch/:roomId" element={<SpectatorPage />} />
          <Route path="/lobby" element={<div>Lobby Landing</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

test('renders the board and redacted player counts with no hand or controls', () => {
  renderSpectator()

  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  // Both players shown with hand counts, but no playable hand of the viewer.
  expect(screen.getByText('Alice')).toBeInTheDocument()
  expect(screen.getByText('Bob')).toBeInTheDocument()
  expect(screen.getByText(/Read-only spectator view/i)).toBeInTheDocument()
  // No play controls.
  expect(screen.queryByRole('button', { name: /Play /i })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /Place face-down/i })).not.toBeInTheDocument()
})

test('shows final results on game over', () => {
  vi.mocked(useSpectatorSocket).mockReturnValue({
    ...liveState,
    gameOver: true,
    results: [
      { player: 'Alice', rank: 1, penalty: 3, winner: true, faceDownCards: [] },
      { player: 'Bob', rank: 2, penalty: 8, winner: false, faceDownCards: [] },
    ],
  })

  renderSpectator()

  expect(screen.getByText(/Final results/i)).toBeInTheDocument()
  expect(screen.getByRole('table', { name: /Score table/i })).toBeInTheDocument()
})

test('shows a not-found message when the game is unavailable', () => {
  vi.mocked(useSpectatorSocket).mockReturnValue({ ...liveState, notFound: true })

  renderSpectator()

  expect(screen.getByText(/isn't available to watch/i)).toBeInTheDocument()
})

test('redirects to your own game instead of spectating it', async () => {
  vi.mocked(getMyActiveRoom).mockResolvedValue({
    active_room: { id: 'room-1', invite_code: 'ABC123', status: 'in_progress', practice_mode: false },
  })

  render(
    <AuthProvider>
      <ActiveRoomProvider>
        <MemoryRouter initialEntries={['/watch/room-1']}>
          <Routes>
            <Route path="/watch/:roomId" element={<SpectatorPage />} />
            <Route path="/game/:roomId" element={<div>Live Game</div>} />
            <Route path="/lobby" element={<div>Lobby Landing</div>} />
          </Routes>
        </MemoryRouter>
      </ActiveRoomProvider>
    </AuthProvider>,
  )

  expect(await screen.findByText('Live Game')).toBeInTheDocument()
})
