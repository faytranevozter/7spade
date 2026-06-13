import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { ActiveGameButton } from './ActiveGameButton'
import { ActiveRoomProvider } from '../hooks/ActiveRoomProvider'
import { AuthProvider } from '../hooks/AuthProvider'
import { getMyActiveRoom, type ActiveRoomDto } from '../api/lobby'

vi.mock('../api/lobby', () => ({
  getMyActiveRoom: vi.fn(),
}))

vi.mock('../api/auth', () => ({
  postRefresh: vi.fn().mockRejectedValue(new Error('no session')),
}))

function LocationProbe() {
  const { pathname } = useLocation()
  return <div data-testid="pathname">{pathname}</div>
}

function renderAt(path: string) {
  return render(
    <AuthProvider>
      <ActiveRoomProvider>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="*" element={<><ActiveGameButton /><LocationProbe /></>} />
          </Routes>
        </MemoryRouter>
      </ActiveRoomProvider>
    </AuthProvider>,
  )
}

const inProgressRoom: ActiveRoomDto = {
  id: 'room-9',
  invite_code: 'ABC123',
  status: 'in_progress',
  practice_mode: false,
}

beforeEach(() => {
  sessionStorage.setItem('seven_spade_auth_token', 'test-token')
})

afterEach(() => {
  cleanup()
  sessionStorage.clear()
  vi.clearAllMocks()
})

test('shows a resume button when the player has an active game and is elsewhere', async () => {
  vi.mocked(getMyActiveRoom).mockResolvedValue({ active_room: inProgressRoom })

  renderAt('/lobby')

  const button = await screen.findByRole('button', { name: /Return to your game/i })
  expect(button).toHaveTextContent('Resume game')
  expect(button).toHaveTextContent('Game in progress')
})

test('navigates to the live game when clicked', async () => {
  vi.mocked(getMyActiveRoom).mockResolvedValue({ active_room: inProgressRoom })

  renderAt('/lobby')

  const button = await screen.findByRole('button', { name: /Return to your game/i })
  fireEvent.click(button)
  await waitFor(() => {
    expect(screen.getByTestId('pathname')).toHaveTextContent('/game/room-9')
  })
})

test('hides itself while already viewing the active room', async () => {
  vi.mocked(getMyActiveRoom).mockResolvedValue({ active_room: inProgressRoom })

  renderAt('/game/room-9')

  // Give the fetch a tick to resolve, then assert the button never appears.
  await waitFor(() => expect(getMyActiveRoom).toHaveBeenCalled())
  expect(screen.queryByRole('button', { name: /Return to your game/i })).not.toBeInTheDocument()
})

test('renders nothing when the player has no active game', async () => {
  vi.mocked(getMyActiveRoom).mockResolvedValue({ active_room: null })

  renderAt('/lobby')

  await waitFor(() => expect(getMyActiveRoom).toHaveBeenCalled())
  expect(screen.queryByRole('button', { name: /Return to your game/i })).not.toBeInTheDocument()
})
