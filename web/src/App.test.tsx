import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import App from './App'
import { deleteLogout, getMe, getOAuthStartUrl, postGuest, postLogin, postOAuthCallback, postRegister } from './api/auth'
import { getHistory } from './api/history'
import { getLeaderboard, getMyStats, getSeasons, getUserStats } from './api/stats'
import { getUserAchievements } from './api/achievements'
import { getLiveGames } from './api/liveGames'
import { getFriends } from './api/friends'
import { getRoom, getRooms, postJoinRoom, postQuickPlay, postRoom } from './api/lobby'

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
  postRefresh: vi.fn(() => Promise.reject(new Error('no session'))),
  postOAuthCallback: vi.fn(),
  getOAuthStartUrl: vi.fn(),
  deleteLogout: vi.fn(),
  getMe: vi.fn(),
  postResendVerification: vi.fn(),
}))

vi.mock('./api/lobby', () => ({
  getRooms: vi.fn(),
  getRoom: vi.fn(),
  postRoom: vi.fn(),
  postJoinRoom: vi.fn(),
  postQuickPlay: vi.fn(),
}))

vi.mock('./api/history', () => ({
  getHistory: vi.fn(),
}))

vi.mock('./api/stats', () => ({
  getLeaderboard: vi.fn(),
  getMyStats: vi.fn(),
  getUserStats: vi.fn(),
  getSeasons: vi.fn(),
  LEADERBOARD_SORTS: [
    { value: 'win_rate', label: 'Win Rate' },
    { value: 'total_wins', label: 'Total Wins' },
    { value: 'avg_penalty', label: 'Avg Penalty' },
    { value: 'best_penalty', label: 'Best Penalty' },
    { value: 'games_played', label: 'Games Played' },
    { value: 'rating', label: 'Rating' },
  ],
  DEFAULT_LEADERBOARD_SORT: 'win_rate',
  isLeaderboardSort: (value: string | null) =>
    ['win_rate', 'total_wins', 'avg_penalty', 'best_penalty', 'games_played', 'rating'].includes(value ?? ''),
}))

vi.mock('./api/achievements', () => ({
  getUserAchievements: vi.fn(),
}))

vi.mock('./api/liveGames', () => ({
  getLiveGames: vi.fn(),
}))

vi.mock('./api/friends', () => ({
  getFriends: vi.fn(),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
  searchUsers: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(getRooms).mockResolvedValue([
    {
      id: 'room-1',
      invite_code: 'XKQP7A',
      visibility: 'public',
      turn_timer_seconds: 60,
      bot_difficulty: 'medium',
      status: 'waiting',
      player_count: 3,
    },
  ])
  vi.mocked(postRoom).mockResolvedValue({
    id: 'new-room-id',
    invite_code: 'NEWCDE',
    visibility: 'public',
    turn_timer_seconds: 60,
    bot_difficulty: 'medium',
    status: 'waiting',
    player_count: 1,
  })
  vi.mocked(postJoinRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    status: 'waiting',
    player_count: 4,
  })
  vi.mocked(postQuickPlay).mockResolvedValue({
    id: 'quick-room',
    invite_code: 'QUICK1',
    status: 'waiting',
    player_count: 1,
  })
  vi.mocked(getRoom).mockResolvedValue({
    id: 'room-1',
    invite_code: 'XKQP7A',
    visibility: 'public',
    turn_timer_seconds: 60,
    bot_difficulty: 'medium',
    status: 'waiting',
    player_count: 1,
  })
  vi.mocked(getHistory).mockResolvedValue({
    games: [
      {
        game_id: 'game-1',
        room_id: 'XKQP7',
        started_at: '2026-05-09T10:00:00Z',
        finished_at: '2026-05-09T10:20:00Z',
        penalty_points: 5,
        rank: 1,
        is_winner: true,
      },
    ],
    total: 1,
    page: 1,
  })
  vi.mocked(getMe).mockResolvedValue({
    user_id: 'me',
    username: 'me',
    display_name: 'Me',
    avatar_url: null,
    created_at: null,
    is_guest: false,
    email_verified: true,
    providers: [],
  })
  vi.mocked(getMyStats).mockResolvedValue({
    user_id: 'me',
    display_name: 'Me',
    avatar_url: null,
    games_played: 10,
    wins: 7,
    win_rate: 0.7,
    avg_penalty: 12.5,
    best_penalty: 3,
    rating: 1240,
    rank: 1,
    qualified: true,
  })
  vi.mocked(getLeaderboard).mockResolvedValue({
    entries: [
      {
        rank: 1,
        user_id: 'leader-1',
        display_name: 'Champion',
        avatar_url: 'https://cdn/champion.png',
        games_played: 20,
        wins: 15,
        win_rate: 0.75,
        avg_penalty: 9.2,
        best_penalty: 2,
        rating: 1380,
      },
    ],
    total: 1,
    page: 1,
    min_games: 5,
    sort: 'win_rate',
    season: '',
  })
  vi.mocked(getSeasons).mockResolvedValue({
    seasons: [
      { id: '2026-06', label: 'June 2026', started_at: '2026-06-01T00:00:00Z', ended_at: null, active: true },
    ],
  })
  vi.mocked(getUserStats).mockResolvedValue({
    user_id: 'leader-1',
    display_name: 'Champion',
    avatar_url: 'https://cdn/champion.png',
    games_played: 20,
    wins: 15,
    win_rate: 0.75,
    avg_penalty: 9.2,
    best_penalty: 2,
    rating: 1380,
    rank: 1,
    qualified: true,
  })
  vi.mocked(getUserAchievements).mockResolvedValue({
    earned: [{ achievement_id: 'first_win', earned_at: '2026-05-09T10:00:00Z' }],
    catalog: [
      { id: 'first_win', name: 'First Blood', description: 'Win your first game', icon: '🏆' },
      { id: 'games_10', name: 'Regular', description: 'Play 10 games', icon: '🎴' },
    ],
  })
  vi.mocked(getLiveGames).mockResolvedValue({ games: [] })
  vi.mocked(getFriends).mockResolvedValue({ friends: [] })
  // Pre-seed an auth token so /lobby renders without redirecting to /auth.
  sessionStorage.setItem('seven_spade_auth_token', 'test-token')
})

afterEach(() => {
  cleanup()
  localStorage.clear()
  sessionStorage.clear()
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
  sessionStorage.clear()
  renderRoute('/auth')
  expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Play as Guest/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Sign In/i })).toBeInTheDocument()
  cleanup()

  sessionStorage.setItem('seven_spade_auth_token', 'test-token')
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
  await waitFor(() => {
    expect(screen.getByText(/XKQP7/i)).toBeInTheDocument()
  })
})

test('renders a single dynamic game route', () => {
  renderRoute('/game/room-1')

  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
})

test('renders the leaderboard route and navigates to a player profile', async () => {
  renderRoute('/leaderboard')

  expect(screen.getByRole('heading', { name: /Leaderboard/i })).toBeInTheDocument()
  await waitFor(() => {
    expect(screen.getByText(/Champion/i)).toBeInTheDocument()
  })

  // The leaderboard row shows the player's avatar image from the DTO.
  const avatar = screen.getByRole('img', { name: /Champion/i })
  expect(avatar).toHaveAttribute('src', 'https://cdn/champion.png')

  fireEvent.click(screen.getByRole('button', { name: /Champion/i }))
  await waitFor(() => {
    expect(screen.getByText(/Player stats/i)).toBeInTheDocument()
  })
  expect(getUserStats).toHaveBeenCalledWith('test-token', 'leader-1')

  // Viewing another player's profile shows the "You vs X" stat comparison.
  await waitFor(() => {
    expect(screen.getByText(/You vs Champion/i)).toBeInTheDocument()
  })
  expect(getMyStats).toHaveBeenCalledWith('test-token')
})

test('history page lets users change rows per page', async () => {
  renderRoute('/history')

  await waitFor(() => {
    expect(getHistory).toHaveBeenCalledWith('test-token', 1, 10)
  })

  fireEvent.change(screen.getByRole('combobox', { name: /Rows/i }), { target: { value: '25' } })

  await waitFor(() => {
    expect(getHistory).toHaveBeenLastCalledWith('test-token', 1, 25)
  })
})

test('leaderboard page lets users change rows per page', async () => {
  renderRoute('/leaderboard')

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenCalledWith('test-token', 1, 10, 'win_rate', '')
  })

  fireEvent.change(screen.getByRole('combobox', { name: /Rows/i }), { target: { value: '25' } })

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenLastCalledWith('test-token', 1, 25, 'win_rate', '')
  })
})

test('leaderboard page sorts by the selected metric and syncs the URL', async () => {
  renderRoute('/leaderboard')

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenCalledWith('test-token', 1, 10, 'win_rate', '')
  })

  fireEvent.change(screen.getByRole('combobox', { name: /Sort leaderboard by/i }), {
    target: { value: 'total_wins' },
  })

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenLastCalledWith('test-token', 1, 10, 'total_wins', '')
  })
})

test('leaderboard page reads the initial sort from the URL', async () => {
  renderRoute('/leaderboard?sort=avg_penalty')

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenCalledWith('test-token', 1, 10, 'avg_penalty', '')
  })

  expect(screen.getByRole('combobox', { name: /Sort leaderboard by/i })).toHaveValue('avg_penalty')
})

test('leaderboard page scopes to the selected season', async () => {
  renderRoute('/leaderboard')

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenCalledWith('test-token', 1, 10, 'win_rate', '')
  })

  fireEvent.change(screen.getByRole('combobox', { name: /Leaderboard season/i }), {
    target: { value: '2026-06' },
  })

  await waitFor(() => {
    expect(getLeaderboard).toHaveBeenLastCalledWith('test-token', 1, 10, 'win_rate', '2026-06')
  })
})

test('temporary buttons navigate through the hardcoded flow', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getAllByRole('button', { name: /^Join$/i })[0])
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Waiting room/i })).toBeInTheDocument()
  })
})

test('quick play finds a room and navigates to the waiting room', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Quick Play/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Quick Play/i }))

  await waitFor(() => {
    expect(postQuickPlay).toHaveBeenCalledWith('test-token')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Waiting room/i })).toBeInTheDocument()
  })
})

test('quick play shows an error when matchmaking fails', async () => {
  vi.mocked(postQuickPlay).mockRejectedValueOnce(new Error('Too many attempts'))
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Quick Play/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Quick Play/i }))

  await waitFor(() => {
    expect(screen.getByRole('alert')).toHaveTextContent('Too many attempts')
  })
})

test('redirects unknown routes to auth', async () => {
  sessionStorage.clear()
  renderRoute('/unknown')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
})

test('redirects login route to auth', async () => {
  sessionStorage.clear()
  renderRoute('/login')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
  expect(screen.getByRole('button', { name: /Sign In/i })).toBeInTheDocument()
})

test('does not render prototype navigation', () => {
  sessionStorage.clear()
  renderRoute('/auth')

  expect(screen.getAllByRole('heading', { name: /SEVEN SPADE/i }).length).toBeGreaterThan(0)
  expect(screen.queryByLabelText(/Prototype scenes/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/Static React\/Tailwind prototype/i)).not.toBeInTheDocument()
})

test('guest submit calls guest auth and navigates to lobby', async () => {
  // Clear pre-seeded token so we exercise the auth flow.
  sessionStorage.clear()
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
  expect(sessionStorage.getItem('seven_spade_auth_token')).toBe('guest-token')
})

test('sign-in submit calls login auth and navigates to lobby', async () => {
  sessionStorage.clear()
  vi.mocked(postLogin).mockResolvedValue({ jwt: 'user-token' })
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
  expect(sessionStorage.getItem('seven_spade_auth_token')).toBe('user-token')
})

test('register route renders create-account form with terms and auth link', () => {
  sessionStorage.clear()
  renderRoute('/register')

  expect(screen.getByRole('heading', { name: /Create Account/i })).toBeInTheDocument()
  expect(screen.getByLabelText(/Display name/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/Username/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/Email/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/^Password$/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/Confirm password/i)).toBeInTheDocument()
  expect(screen.getByLabelText(/I agree to the/i)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Sign In/i })).toHaveAttribute('href', '/auth')
})

test('register submit stays blocked until fields and terms are valid', async () => {
  sessionStorage.clear()
  vi.mocked(postRegister).mockResolvedValue({ jwt: 'new-user-token' })
  renderRoute('/register')

  const submitButton = screen.getByRole('button', { name: /Create Account/i })
  expect(submitButton).toBeDisabled()

  fireEvent.change(screen.getByLabelText(/Display name/i), { target: { value: 'New Player' } })
  fireEvent.change(screen.getByLabelText(/Username/i), { target: { value: 'new_player' } })
  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'new@example.com' } })
  fireEvent.change(screen.getByLabelText(/^Password$/i), { target: { value: 'password123' } })
  fireEvent.change(screen.getByLabelText(/Confirm password/i), { target: { value: 'password123' } })

  expect(submitButton).toBeDisabled()
  fireEvent.click(screen.getByLabelText(/I agree to the/i))
  expect(submitButton).toBeEnabled()

  fireEvent.click(submitButton)

  await waitFor(() => {
    expect(postRegister).toHaveBeenCalledWith('new@example.com', 'password123', 'New Player', 'new_player')
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
  fireEvent.click(screen.getByRole('button', { name: /^Create$/i }))

  await waitFor(() => {
    expect(postRoom).toHaveBeenCalledWith('test-token', {
      visibility: 'public',
      turn_timer_seconds: 60,
      bot_difficulty: 'medium',
    })
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Waiting room/i })).toBeInTheDocument()
  })
})

test('lobby starts a practice game as a private bots-only room', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /^Practice$/i }))
  fireEvent.click(screen.getByRole('button', { name: /Start practice/i }))

  await waitFor(() => {
    expect(postRoom).toHaveBeenCalledWith('test-token', {
      visibility: 'private',
      turn_timer_seconds: 60,
      bot_difficulty: 'medium',
      practice_mode: true,
    })
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Waiting room/i })).toBeInTheDocument()
  })
})

test('lobby joins by invite code and navigates to that game', async () => {
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Join by code/i }))
  fireEvent.change(screen.getByLabelText(/Invite code/i), { target: { value: 'xkqp7a' } })
  fireEvent.click(screen.getByRole('button', { name: /Join with code/i }))

  await waitFor(() => {
    expect(postJoinRoom).toHaveBeenCalledWith('test-token', 'XKQP7A')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Waiting room/i })).toBeInTheDocument()
  })
})

test('lobby surfaces an error message when room creation fails', async () => {
  vi.mocked(postRoom).mockRejectedValueOnce(new Error('Server unhappy'))
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Create room/i }))
  fireEvent.click(screen.getByRole('button', { name: /^Create$/i }))

  await waitFor(() => {
    expect(screen.getByRole('alert')).toHaveTextContent(/Server unhappy/i)
  })
})

test('lobby redirects unauthenticated users to auth', async () => {
  sessionStorage.clear()
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
})

test('redirects authenticated users away from the auth page to the lobby', async () => {
  // Token seeded in beforeEach — visiting /auth while logged in lands on /lobby.
  renderRoute('/auth')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(screen.queryByRole('heading', { name: /Take Your Seat/i })).not.toBeInTheDocument()
})

test('redirects authenticated users away from the register page to the lobby', async () => {
  renderRoute('/register')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(screen.queryByRole('heading', { name: /Create Account/i })).not.toBeInTheDocument()
})

test('sign out clears the session and returns to the auth page', async () => {
  vi.mocked(deleteLogout).mockResolvedValue(undefined)
  renderRoute('/lobby')

  await waitFor(() => {
    expect(screen.getByText(/XKQP7A/i)).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Sign out/i }))

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Take Your Seat/i })).toBeInTheDocument()
  })
  expect(deleteLogout).toHaveBeenCalled()
  expect(sessionStorage.getItem('seven_spade_auth_token')).toBeNull()
})

test('auth page renders Google and GitHub OAuth buttons', () => {
  sessionStorage.clear()
  renderRoute('/auth')

  expect(screen.getByRole('button', { name: /Continue with Google/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Continue with GitHub/i })).toBeInTheDocument()
})

test('clicking Google OAuth button navigates to backend start URL', async () => {
  sessionStorage.clear()
  vi.mocked(getOAuthStartUrl).mockResolvedValue({ url: 'https://accounts.google.com/auth', state: 'state-1' })
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
    await waitFor(() => {
      expect(getOAuthStartUrl).toHaveBeenCalledWith('google')
      expect(assignSpy).toHaveBeenCalledWith('https://accounts.google.com/auth')
    })
  } finally {
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
  }
})

test('OAuth callback route with provider posts code/state, stores token, and redirects to lobby', async () => {
  vi.mocked(postOAuthCallback).mockResolvedValue({ jwt: 'oauth-jwt' })

  render(
    <MemoryRouter initialEntries={[{ pathname: '/auth/callback/github', search: '?code=code-1&state=state-1' }]}> 
      <App />
    </MemoryRouter>,
  )

  await waitFor(() => {
    expect(postOAuthCallback).toHaveBeenCalledWith('github', 'code-1', 'state-1')
  })
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  })
  expect(sessionStorage.getItem('seven_spade_auth_token')).toBe('oauth-jwt')
})

test('OAuth callback shows error message on failure', async () => {
  sessionStorage.clear()
  render(
    <MemoryRouter initialEntries={[{ pathname: '/auth/callback/google', search: '?error=access_denied' }]}> 
      <App />
    </MemoryRouter>,
  )

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Sign-in failed/i })).toBeInTheDocument()
  })
  expect(screen.getByText(/cancelled the sign-in/i)).toBeInTheDocument()
  expect(sessionStorage.getItem('seven_spade_auth_token')).toBeNull()
})
