import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import App from './App'
import { postGuest, postLogin, postRegister, postTelegramAuth } from './api/auth'
import { getRooms, postJoinRoom, postRoom } from './api/lobby'

vi.mock('./api/auth', () => ({
  AuthApiError: class AuthApiError extends Error {
    statusCode: number

    constructor(message: string, statusCode: number) {
      super(message)
      this.name = 'AuthApiError'
      this.statusCode = statusCode
    }
  },
  postGuest: vi.fn(),
  postLogin: vi.fn(),
  postRegister: vi.fn(),
  postTelegramAuth: vi.fn(),
  getOAuthStartUrl: (provider: string) => `http://localhost:8080/auth/${provider}`,
  parseOAuthCallbackFragment: (fragment: string) => {
    const cleaned = fragment.startsWith('#') ? fragment.slice(1) : fragment
    const params = new URLSearchParams(cleaned)
    return {
      provider: params.get('provider') ?? '',
      jwt: params.get('jwt') ?? undefined,
      refreshToken: params.get('refresh_token') ?? undefined,
      error: params.get('error') ?? undefined,
    }
  },
}))

vi.mock('./api/lobby', () => ({
  getRooms: vi.fn(),
  postRoom: vi.fn(),
  postJoinRoom: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(getRooms).mockResolvedValue([
    {
      id: 'room-1',
      invite_code: 'XKQP7A',
      visibility: 'public',
      turn_timer_seconds: 60,
      status: 'waiting',
      player_count: 3,
    },
  ])
  vi.mocked(postRoom).mockResolvedValue({
    id: 'new-room-id',
    invite_code: 'NEWCDE',
    visibility: 'public',
    turn_timer_seconds: 60,
    status: 'waiting',
    player_count: 1,
  })
  vi.mocked(postJoinRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    status: 'waiting',
    player_count: 4,
  })
  // Pre-seed an auth token so /lobby renders without redirecting to /auth.
  localStorage.setItem('seven_spade_auth_token', 'test-token')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.clearAllMocks()
})

function renderRoute(route: string) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <App />
    </MemoryRouter>,
  )
}

test('renders real top-level routes with temporary hardcoded data', async () => {
  renderRoute('/auth')
  expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Play as Guest/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Sign In/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/lobby')
  expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })
  cleanup()

  renderRoute('/results/room-1')
  expect(screen.getByRole('table', { name: /Score table/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/history')
  expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
  expect(screen.getByText(/XKQP7/i)).toBeInTheDocument()
})

test('renders a single dynamic game route', () => {
  renderRoute('/game/room-1')

  expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Play card/i })).toBeInTheDocument()
})

test('temporary buttons navigate through the hardcoded flow', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getAllByRole('button', { name: /^Join$/i })[0])
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Play card/i }))
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Results and rematch/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /View history/i }))
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
  })
})

test('redirects unknown routes to auth', async () => {
  renderRoute('/unknown')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
})

test('redirects login route to auth', async () => {
  renderRoute('/login')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
  expect(screen.getByRole('button', { name: /Sign In/i })).toBeInTheDocument()
})

test('does not render prototype navigation', () => {
  renderRoute('/auth')

  expect(screen.getAllByRole('heading', { name: /SEVEN SPADE/i }).length).toBeGreaterThan(0)
  expect(screen.queryByLabelText(/Prototype scenes/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/Static React\/Tailwind prototype/i)).not.toBeInTheDocument()
})

test('guest submit calls guest auth and navigates to lobby', async () => {
  // Clear pre-seeded token so we exercise the auth flow.
  localStorage.clear()
  vi.mocked(postGuest).mockResolvedValue({ token: 'guest-token' })
  renderRoute('/auth')

  fireEvent.change(screen.getByLabelText(/Display name/i), { target: { value: 'Guest Player' } })
  fireEvent.click(screen.getByRole('button', { name: /^Continue$/i }))

  await waitFor(() => {
    expect(postGuest).toHaveBeenCalledWith('Guest Player')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(localStorage.getItem('seven_spade_auth_token')).toBe('guest-token')
})

test('sign-in submit calls login auth and navigates to lobby', async () => {
  localStorage.clear()
  vi.mocked(postLogin).mockResolvedValue({ jwt: 'user-token', refresh_token: 'refresh-token' })
  renderRoute('/auth')

  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'player@example.com' } })
  fireEvent.change(screen.getByLabelText(/Password/i), { target: { value: 'password123' } })
  fireEvent.click(screen.getByRole('button', { name: /Sign In/i }))

  await waitFor(() => {
    expect(postLogin).toHaveBeenCalledWith('player@example.com', 'password123')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(localStorage.getItem('seven_spade_auth_token')).toBe('user-token')
  expect(localStorage.getItem('seven_spade_refresh_token')).toBe('refresh-token')
})

test('telegram auth callback posts payload and navigates to lobby', async () => {
  vi.mocked(postTelegramAuth).mockResolvedValue({ jwt: 'telegram-token', refresh_token: 'telegram-refresh-token' })
  renderRoute('/auth')

  window.onTelegramAuth?.({ id: 123, first_name: 'Ada', auth_date: 1710000000, hash: 'valid-hash' })

  await waitFor(() => {
    expect(postTelegramAuth).toHaveBeenCalledWith({ id: 123, first_name: 'Ada', auth_date: 1710000000, hash: 'valid-hash' })
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(localStorage.getItem('seven_spade_auth_token')).toBe('telegram-token')
  expect(localStorage.getItem('seven_spade_refresh_token')).toBe('telegram-refresh-token')
})

test('register route renders create-account form with terms and auth link', () => {
  renderRoute('/register')

  expect(screen.getByRole('heading', { name: /Create Account/i })).toBeInTheDocument()
  expect(screen.getByLabelText(/Display name/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/Email/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/^Password$/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/Confirm password/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/I agree to the/i)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Sign In/i })).toHaveAttribute('href', '/auth')
})

test('register submit stays blocked until fields and terms are valid', async () => {
  localStorage.clear()
  vi.mocked(postRegister).mockResolvedValue({ jwt: 'new-user-token', refresh_token: 'new-refresh-token' })
  renderRoute('/register')

  const submitButton = screen.getByRole('button', { name: /Create Account/i })
  expect(submitButton).toBeDisabled()

  fireEvent.change(screen.getByLabelText(/Display name/i), { target: { value: 'New Player' } })
  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'new@example.com' } })
  fireEvent.change(screen.getByLabelText(/^Password$/i), { target: { value: 'password123' } })
  fireEvent.change(screen.getByLabelText(/Confirm password/i), { target: { value: 'password123' } })

  expect(submitButton).toBeDisabled()
  fireEvent.click(screen.getByLabelText(/I agree to the/i))
  expect(submitButton).toBeEnabled()

  fireEvent.click(submitButton)

  await waitFor(() => {
    expect(postRegister).toHaveBeenCalledWith('new@example.com', 'password123', 'New Player')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
})

test('lobby creates a room and navigates to the new game', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Create room/i }))

  await waitFor(() => {
    expect(postRoom).toHaveBeenCalledWith('test-token', {
      visibility: 'public',
      turn_timer_seconds: 60,
    })
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  })
})

test('lobby joins by invite code and navigates to that game', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.change(screen.getByLabelText(/Invite code/i), { target: { value: 'xkqp7a' } })
  fireEvent.click(screen.getByRole('button', { name: /Join with code/i }))

  await waitFor(() => {
    expect(postJoinRoom).toHaveBeenCalledWith('test-token', 'XKQP7A')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  })
})

test('lobby surfaces an error message when room creation fails', async () => {
  vi.mocked(postRoom).mockRejectedValueOnce(new Error('Server unhappy'))
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Create room/i }))

  await waitFor(() => {
    expect(screen.getByRole('alert')).toHaveTextContent(/Server unhappy/i)
  })
})

test('lobby redirects unauthenticated users to auth', async () => {
  localStorage.clear()
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
})

test('auth page renders Google and GitHub OAuth buttons', () => {
  renderRoute('/auth')

  expect(screen.getByRole('button', { name: /Continue with Google/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Continue with GitHub/i })).toBeInTheDocument()
})

test('clicking Google OAuth button navigates to backend start URL', () => {
  renderRoute('/auth')

  const originalLocation = window.location
  const assignSpy = vi.fn()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      ...originalLocation,
      assign: assignSpy,
      set href(value: string) {
        assignSpy(value)
      },
    },
  })

  try {
    fireEvent.click(screen.getByRole('button', { name: /Continue with Google/i }))
    expect(assignSpy).toHaveBeenCalledWith('http://localhost:8080/auth/google')
  } finally {
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
  }
})

test('OAuth callback stores tokens and redirects to lobby', async () => {
  // Set the URL fragment to simulate the backend redirect
  window.history.replaceState(null, '', '/auth/callback#provider=google&jwt=oauth-jwt&refresh_token=oauth-refresh')

  render(
    <MemoryRouter initialEntries={[{ pathname: '/auth/callback', hash: '#provider=google&jwt=oauth-jwt&refresh_token=oauth-refresh' }]}>
      <App />
    </MemoryRouter>,
  )

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(localStorage.getItem('seven_spade_auth_token')).toBe('oauth-jwt')
  expect(localStorage.getItem('seven_spade_refresh_token')).toBe('oauth-refresh')
})

test('OAuth callback shows error message on failure', async () => {
  render(
    <MemoryRouter initialEntries={[{ pathname: '/auth/callback', hash: '#provider=google&error=access_denied' }]}>
      <App />
    </MemoryRouter>,
  )

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Sign-in failed/i })).toBeInTheDocument()
  })
  expect(screen.getByText(/cancelled the sign-in/i)).toBeInTheDocument()
  expect(localStorage.getItem('seven_spade_auth_token')).toBeNull()
})
})
