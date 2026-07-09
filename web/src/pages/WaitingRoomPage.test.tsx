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
  getMyActiveRoom: vi.fn().mockResolvedValue({ active_room: null }),
}))

const sendLeave = vi.fn()
const sendStartGame = vi.fn()
const sendSetReady = vi.fn()
const sendKick = vi.fn()

function baseState(lobby: LobbyState): GameSocketState {
  return {
    status: 'open',
    phase: 'lobby',
    lobby,
    isHost: true,
    iAmReady: true,
    boardRows: [],
    hand: [],
    myFaceDown: [],
    players: [],
    toasts: [],
    isMyTurn: false,
    currentTurnName: null,
    turnEndsAt: null,
    turnTimerSeconds: 60,
    rematchVotes: 0,
    rematchTotal: 4,
    rematchEndsAt: null,
    roomClosed: false,
    gameOver: false,
    results: [],
    practiceMode: false,
    teamInfo: null,
    emotes: {},
    spectatorReactions: [],
    myDisplayName: 'Tester',
    sendPlayCard: vi.fn(),
    sendFaceDown: vi.fn(),
    sendRematchVote: vi.fn(),
    sendGoToWaitingRoom: vi.fn(),
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendKick,
    sendEmote: vi.fn(),
    sendSetTeam: vi.fn(),
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
    bot_difficulty: 'medium',
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
        { displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false },
        { displayName: 'Bob', slot: 1, isHost: false, ready: true, disconnected: true },
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
        { displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false },
        { displayName: 'Bob', slot: 1, isHost: false, ready: true, disconnected: false },
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
      players: [{ displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false }],
    }),
  )

  renderWaiting()

  fireEvent.click(screen.getByRole('button', { name: /Leave room/i }))
  expect(sendLeave).toHaveBeenCalledOnce()
})

test('duplicate display names do not make a non-host look like the host', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...baseState({
      hostDisplayName: 'Alex',
      yourSlot: 1,
      minToStart: 2,
      maxPlayers: 4,
      canStart: false,
      players: [
        { displayName: 'Alex', slot: 0, isHost: true, ready: true, disconnected: false },
        { displayName: 'Alex', slot: 1, isHost: false, ready: false, disconnected: false },
      ],
    }),
    isHost: false,
    iAmReady: false,
    myDisplayName: 'Alex',
  })

  renderWaiting()

  expect(screen.queryByRole('button', { name: /Start game/i })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Mark ready/i })).toBeEnabled()
  expect(screen.getAllByText('Alex')).toHaveLength(2)
})

test('host can kick a non-host player', () => {
  vi.mocked(useGameSocket).mockReturnValue(
    baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: true,
      players: [
        { displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false },
        { displayName: 'Bob', slot: 1, isHost: false, ready: true, disconnected: false },
      ],
    }),
  )

  renderWaiting()

  const kickButtons = screen.getAllByRole('button', { name: /Remove .* from the room/i })
  expect(kickButtons).toHaveLength(1)
  fireEvent.click(kickButtons[0])
  expect(sendKick).toHaveBeenCalledWith(1)
})

test('non-host sees no kick buttons', () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: true,
      players: [
        { displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false },
        { displayName: 'Tester', slot: 1, isHost: false, ready: true, disconnected: false },
      ],
    }),
    isHost: false,
  })

  renderWaiting()

  expect(screen.queryByRole('button', { name: /Remove .* from the room/i })).not.toBeInTheDocument()
})

test('a kicked player (room_closed) is sent back to the lobby', async () => {
  vi.mocked(useGameSocket).mockReturnValue({
    ...baseState({
      hostDisplayName: 'Alice',
      minToStart: 2,
      maxPlayers: 4,
      canStart: false,
      players: [{ displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false }],
    }),
    isHost: false,
    roomClosed: true,
  })

  renderWaiting()

  expect(await screen.findByText('Lobby Landing')).toBeInTheDocument()
})

test('practice room shows a Practice badge and a Start practice button', () => {
  vi.mocked(getRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    visibility: 'private',
    turn_timer_seconds: 60,
    bot_difficulty: 'medium',
    practice_mode: true,
    status: 'waiting',
    player_count: 1,
  })
  vi.mocked(useGameSocket).mockReturnValue({
    ...baseState({
      hostDisplayName: 'Alice',
      minToStart: 1,
      maxPlayers: 4,
      canStart: true,
      players: [{ displayName: 'Alice', slot: 0, isHost: true, ready: true, disconnected: false }],
    }),
    practiceMode: true,
  })

  renderWaiting()

  expect(screen.getAllByText('Practice').length).toBeGreaterThan(0)
  expect(screen.getByRole('button', { name: /Start practice/i })).toBeEnabled()
  // Invite-sharing controls are hidden for solo practice rooms.
  expect(screen.queryByRole('button', { name: /Invite a friend/i })).not.toBeInTheDocument()
})
