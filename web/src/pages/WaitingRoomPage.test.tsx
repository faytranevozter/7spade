import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { WaitingRoomPage } from './WaitingRoomPage'
import { AuthProvider } from '../hooks/AuthProvider'
import { useGameSocket, type GameSocketState, type LobbyState } from '../hooks/useGameSocket'
import { getRoom } from '../api/lobby'

vi.mock('../hooks/useGameSocket', () => ({
  useGameSocket: vi.fn(),
}))

vi.mock('../api/lobby', () => ({
  getRoom: vi.fn(),
}))

const sendLeave = vi.fn()
const sendStartGame = vi.fn()
const sendSetReady = vi.fn()

function baseState(lobby: LobbyState): GameSocketState {
  return {
    status: 'open',
    phase: 'lobby',
    lobby,
    isHost: true,
    iAmReady: true,
    boardRows: [],
    hand: [],
    players: [],
    toasts: [],
    isMyTurn: false,
    currentTurnName: null,
    turnEndsAt: null,
    rematchVotes: 0,
    rematchTotal: 4,
    gameOver: false,
    results: [],
    emotes: {},
    myDisplayName: 'Tester',
    sendPlayCard: vi.fn(),
    sendFaceDown: vi.fn(),
    sendRematchVote: vi.fn(),
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendEmote: vi.fn(),
    reconnect: vi.fn(),
  }
}

beforeEach(() => {
  sessionStorage.setItem('seven_spade_auth_token', 'test-token')
  vi.mocked(getRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    visibility: 'public',
    turn_timer_seconds: 60,
    status: 'waiting',
    player_count: 2,
  })
})

afterEach(() => {
  cleanup()
  sessionStorage.clear()
  vi.clearAllMocks()
})

function renderWaiting() {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={['/room/room-1']}>
        <Routes>
          <Route path="/room/:roomId" element={<WaitingRoomPage />} />
          <Route path="/lobby" element={<div>Lobby Landing</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  )
}

test('shows a Disconnected badge and disables Start when a player has dropped', () => {
  vi.mocked(useGameSocket).mockReturnValue(
    baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: false,
      players: [
        { displayName: 'Alice', isHost: true, ready: true, disconnected: false },
        { displayName: 'Bob', isHost: false, ready: true, disconnected: true },
      ],
    }),
  )

  renderWaiting()

  expect(screen.getByText('Disconnected')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Start game/i })).toBeDisabled()
})

test('enables Start when all connected players are ready', () => {
  vi.mocked(useGameSocket).mockReturnValue(
    baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: true,
      players: [
        { displayName: 'Alice', isHost: true, ready: true, disconnected: false },
        { displayName: 'Bob', isHost: false, ready: true, disconnected: false },
      ],
    }),
  )

  renderWaiting()

  expect(screen.queryByText('Disconnected')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Start game/i })).toBeEnabled()
})

test('Leave room notifies the server before navigating away', () => {
  vi.mocked(useGameSocket).mockReturnValue(
    baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: true,
      players: [{ displayName: 'Alice', isHost: true, ready: true, disconnected: false }],
    }),
  )

  renderWaiting()

  fireEvent.click(screen.getByRole('button', { name: /Leave room/i }))
  expect(sendLeave).toHaveBeenCalledOnce()
})
