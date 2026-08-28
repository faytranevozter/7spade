import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  window.location.hash = ''
  vi.restoreAllMocks()
})

test('page reload refreshes a rotating session only once in StrictMode', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: [] }
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<StrictMode><App /></StrictMode>)

  expect(await screen.findByText('Operations overview')).toBeInTheDocument()
  expect(fetchMock.mock.calls.filter(([input]) => String(input).endsWith('/auth/refresh'))).toHaveLength(1)
})

test('administrator signs in and sees the protected dashboard', async () => {
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin: { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['dashboard.read'] } }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({
    status: 'ready', environment: 'staging',
    windows: { day: { from: '2026-08-28T00:00:00Z', to: '2026-08-29T00:00:00Z' }, month: { from: '2026-08-01T00:00:00Z', to: '2026-09-01T00:00:00Z' } },
    current: { players: 12, rooms: 3, games: 2 },
    daily: { registrations: 4, players: 12, rooms: 3, games_started: 2, games_completed: 1, games_abandoned: 0, average_game_duration_seconds: 125 },
    monthly: { registrations: 20, players: 33, rooms: 15, games_started: 13, games_completed: 12, games_abandoned: 1, average_game_duration_seconds: 245 },
    services: { api: { status: 'ok' }, ws: { status: 'degraded' } },
    links: [{ name: 'metrics', url: 'https://metrics.example.com' }],
  }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([]), { status: 200 }))
  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Admin sign in' })).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ops@example.com' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(await screen.findByText('Operations overview')).toBeInTheDocument()
  expect(await screen.findByText('STAGING')).toBeInTheDocument()
  expect(await screen.findByText('Active players')).toBeInTheDocument()
  expect(await screen.findByText('DEGRADED')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'metrics' })).toHaveAttribute('href', 'https://metrics.example.com')
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(4))
})

test('operator searches a player and inspects redacted progression data', async () => {
  window.location.hash = '#users'
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['users.read'] }
  const user = { id: '00000000-0000-0000-0000-000000000001', username: 'ace', display_name: 'Ace Player', created_at: '2026-08-01T00:00:00Z', online: true }
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.includes('/users?')) return new Response(JSON.stringify({ users: [user], limit: 50, offset: 0 }), { status: 200 })
    if (url.endsWith(`/users/${user.id}`)) return new Response(JSON.stringify({ user, providers: ['google'], stats: { xp: 250 }, ratings: [], achievements: [], skins: [], games: [] }), { status: 200 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
  fireEvent.click(await screen.findByRole('link', { name: /Ace Player/ }))
  expect(await screen.findByRole('heading', { name: 'Ace Player' })).toBeInTheDocument()
  expect(screen.getByText('xp: 250')).toBeInTheDocument()
  expect(screen.queryByText('ops@example.com')).not.toBeInTheDocument()
  expect(fetchMock).toHaveBeenCalled()
})

test('operator searches a room and sees durable and unavailable live state', async () => {
  window.location.hash = '#/rooms'
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['rooms.read'] }
  const room = { id: '10000000-0000-0000-0000-000000000001', invite_code: 'ACE123', name: 'Practice table', status: 'waiting', visibility: 'private', game_mode: 'classic', practice_mode: true, max_players: 4, deck_count: 1, scoring_mode: 'rank_value', team_mode: 'ffa', turn_timer_seconds: 60, created_by: 'owner-1', created_at: '2026-08-20T12:00:00Z', player_count: 1 }
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.includes('/rooms?')) return new Response(JSON.stringify({ rooms: [room], limit: 50, offset: 0 }), { status: 200 })
    if (url.endsWith(`/rooms/${room.id}`)) return new Response(JSON.stringify({ room, players: [{ user_id: 'player-1', display_name: 'Ace', joined_at: '2026-08-20T12:00:00Z' }], live: { available: false, reason: 'unavailable' } }), { status: 200 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Rooms' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Rooms' })).toBeInTheDocument()
  fireEvent.click(await screen.findByRole('link', { name: /Practice table/ }))
  expect(await screen.findByRole('heading', { name: 'Practice table' })).toBeInTheDocument()
  expect(screen.getByText('Live room state unavailable: unavailable.')).toBeInTheDocument()
  expect(fetchMock.mock.calls.some(([input]) => String(input).includes('/rooms?limit=50&offset=0'))).toBe(true)
})

test('operator inspects a redacted live summary including bot seats', async () => {
  window.location.hash = '#/rooms/10000000-0000-0000-0000-000000000001'
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['rooms.read'] }
  const room = { id: '10000000-0000-0000-0000-000000000001', invite_code: 'ACE123', name: 'Practice table', status: 'in_progress', visibility: 'public', game_mode: 'classic', practice_mode: false, max_players: 4, deck_count: 1, scoring_mode: 'rank_value', team_mode: 'ffa', turn_timer_seconds: 60, created_by: 'owner-1', created_at: '2026-08-20T12:00:00Z', player_count: 2 }
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.endsWith(`/rooms/${room.id}`)) return new Response(JSON.stringify({ room, players: [{ user_id: 'player-1', display_name: 'Ace', joined_at: '2026-08-20T12:00:00Z' }], live: { available: true, summary: { role: 'owner', phase: 'playing', players: [{ user_id: 'player-1', display_name: 'Ace', connected: true }, { user_id: 'bot-1', display_name: 'Robo', connected: true, is_bot: true }], snapshot_age_seconds: 3, state_version: 7, owner_id: 'ws-2', fence_token: 9 } } }), { status: 200 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Practice table' })).toBeInTheDocument()
  expect(await screen.findByText(/Phase: playing/)).toBeInTheDocument()
  expect(screen.getByText(/Robo · connected · bot/)).toBeInTheDocument()
  expect(screen.queryByText(/hidden/)).not.toBeInTheDocument()
})

test('operator opens user detail and advances user pagination', async () => {
  window.location.hash = '#/users'
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['users.read'] }
  const firstPage = Array.from({ length: 50 }, (_, index) => ({
    id: `00000000-0000-0000-0000-${String(index + 1).padStart(12, '0')}`,
    username: `player${index + 1}`,
    display_name: `Player ${index + 1}`,
    created_at: '2026-08-01T00:00:00Z',
    online: false,
  }))
  const nextUser = { id: '00000000-0000-0000-0000-000000000051', username: 'player51', display_name: 'Player 51', created_at: '2026-08-01T00:00:00Z', online: true }
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.includes('/users?limit=50&offset=0')) return new Response(JSON.stringify({ users: firstPage, limit: 50, offset: 0 }), { status: 200 })
    if (url.includes('/users?limit=50&offset=50')) return new Response(JSON.stringify({ users: [nextUser], limit: 50, offset: 50 }), { status: 200 })
    if (url.endsWith(`/users/${firstPage[0].id}`)) return new Response(JSON.stringify({ user: firstPage[0], providers: null, stats: { xp: 10 }, ratings: null, achievements: null, skins: null, games: null }), { status: 200 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  fireEvent.click(await screen.findByRole('link', { name: /Player 1 @player1/ }))
  expect(await screen.findByRole('heading', { name: 'Player 1' })).toBeInTheDocument()
  expect(window.location.hash).toBe(`#/users/${firstPage[0].id}`)
  fireEvent.click(screen.getByRole('link', { name: 'Back to users' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Next' }))
  expect(await screen.findByRole('link', { name: /Player 51 @player51/ })).toBeInTheDocument()
  expect(fetchMock.mock.calls.some(([input]) => String(input).includes('/users?limit=50&offset=50'))).toBe(true)
})

test('moderator recovers a display name conflict without losing the attempted replacement', async () => {
  window.location.hash = '#/users/00000000-0000-0000-0000-000000000001'
  const admin = { id: '1', email: 'mod@example.com', display_name: 'Moderator', status: 'active', permissions: ['users.read', 'users.moderate'] }
  const original = { id: '00000000-0000-0000-0000-000000000001', username: 'ace', display_name: 'Bad Name', version: 3, created_at: '2026-08-01T00:00:00Z', online: true }
  const current = { ...original, display_name: 'Newer Name', version: 4 }
  let detailCalls = 0
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.endsWith(`/users/${original.id}`) && !init?.method) {
      detailCalls += 1
      return new Response(JSON.stringify({ user: detailCalls === 1 ? original : current, providers: [], stats: {}, ratings: [], achievements: [], skins: [], games: [] }), { status: 200 })
    }
    if (url.endsWith(`/users/${original.id}/display-name`)) return new Response(JSON.stringify({ error: 'User changed since it was loaded' }), { status: 409 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  fireEvent.change(await screen.findByLabelText('Replacement display name'), { target: { value: 'Clean Name' } })
  fireEvent.change(screen.getByLabelText('Moderation reason'), { target: { value: 'Inappropriate name' } })
  fireEvent.click(screen.getByRole('button', { name: 'Replace display name' }))

  expect(await screen.findByRole('alert')).toHaveTextContent('Current name: Newer Name')
  expect(screen.getByLabelText('Replacement display name')).toHaveValue('Clean Name')
  const updateCall = fetchMock.mock.calls.find(([input]) => String(input).endsWith('/display-name'))
  expect(updateCall?.[1]?.body).toBe(JSON.stringify({ display_name: 'Clean Name', reason: 'Inappropriate name', version: 3 }))
})

test('administrator completes an MFA challenge before entering the shell', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: [] }
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ mfa_required: true, challenge_token: 'challenge' }), { status: 202 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 }))
  render(<App />)
  fireEvent.change(await screen.findByLabelText('Email'), { target: { value: 'ops@example.com' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
  fireEvent.change(await screen.findByLabelText('Authentication code'), { target: { value: '123456' } })
  fireEvent.click(screen.getByRole('button', { name: 'Verify' }))
  expect(await screen.findByText('Operations overview')).toBeInTheDocument()
  expect(fetchMock.mock.calls[2]?.[1]?.body).toBe(JSON.stringify({ challenge_token: 'challenge', code: '123456' }))
})

test('expired access is refreshed and the protected request is retried', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['dashboard.read'] }
  let dashboardCalls = 0
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: dashboardCalls ? 'fresh' : 'expired', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.endsWith('/dashboard')) {
      dashboardCalls += 1
      if (dashboardCalls === 1) return new Response(JSON.stringify({ error: 'Authentication required' }), { status: 401 })
      expect(init?.headers).toEqual({ Authorization: 'Bearer fresh' })
      return new Response(JSON.stringify({ status: 'ready', environment: 'production' }), { status: 200 })
    }
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)

  expect(await screen.findByText('PRODUCTION')).toBeInTheDocument()
  expect(fetchMock).toHaveBeenCalled()
})

test('administrator revokes another active session', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: [] }
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([
    { id: 'current', created_at: '2026-08-27T10:00:00Z', expires_at: '2026-09-27T10:00:00Z', ip_address: '127.0.0.1', user_agent: 'This browser', current: true },
    { id: 'other', created_at: '2026-08-26T10:00:00Z', expires_at: '2026-09-26T10:00:00Z', ip_address: '192.0.2.1', user_agent: 'Lost laptop', current: false },
  ]), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Revoke' }))

  expect(await screen.findByRole('status')).toHaveTextContent('Session revoked')
  expect(screen.queryByText('Lost laptop')).not.toBeInTheDocument()
})

test('failed logout clears local access and reports the failure', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: [] }
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.endsWith('/auth/logout')) return new Response(JSON.stringify({ error: 'Sign out failed' }), { status: 500 })
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Sign out' }))

  expect(await screen.findByRole('heading', { name: 'Admin sign in' })).toBeInTheDocument()
  expect(screen.getByRole('alert')).toHaveTextContent('Sign out failed')
})

test('super administrator manages other administrators', async () => {
  const superAdmin = {
    id: 'super-1',
    email: 'super@example.com',
    display_name: 'Super Admin',
    status: 'active',
    permissions: ['dashboard.read', 'admins.read', 'admins.manage'],
  }
  const otherAdmin = {
    id: 'mod-1',
    email: 'mod@example.com',
    display_name: 'Moderator Admin',
    status: 'active',
    roles: [{ id: 'role-mod', name: 'moderator', description: '', permissions: [] }],
    permissions: ['dashboard.read'],
  }
  const roles = [
    { id: 'role-mod', name: 'moderator', description: '', permissions: [] },
    { id: 'role-op', name: 'operator', description: '', permissions: [] },
  ]

  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin: superAdmin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ status: 'ready', environment: 'production' }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([]), { status: 200 })) // sessions
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([superAdmin, otherAdmin]), { status: 200 })) // getAdmins
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify(roles), { status: 200 })) // getRoles
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([
    { name: 'dashboard.read', description: 'View dashboard' },
    { name: 'users.moderate', description: 'Moderate users' },
  ]), { status: 200 })) // getPermissions

  // invite
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ invitation: { email: 'new@example.com' }, token: 'secret-token-123' }), { status: 201 }))
  // status toggle
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
  // role change
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
  // permission mapping change
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

  render(<App />)

  expect(await screen.findByRole('heading', { name: 'Operations overview' })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: 'Administrators' })).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('link', { name: 'Administrators' }))
  expect(await screen.findByRole('heading', { name: 'Administrators' })).toBeInTheDocument()
  expect(await screen.findByText('Moderator Admin')).toBeInTheDocument()

  // 1. Send invite
  fireEvent.change(screen.getByLabelText('Invite Email'), { target: { value: 'new@example.com' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send Invite' }))

  expect(await screen.findByText('Token: secret-token-123')).toBeInTheDocument()
  expect(screen.getByRole('status')).toHaveTextContent('Invitation created for new@example.com')

  // 2. Change role
  fireEvent.change(screen.getByLabelText('Role for Moderator Admin'), { target: { value: 'role-op' } })
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Administrator role updated'))

  // 3. Disable admin
  fireEvent.click(screen.getByRole('button', { name: 'Disable' }))
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Administrator mod@example.com is now disabled'))

  // 4. Update a role's permission mapping
  fireEvent.click(screen.getByLabelText('users.moderate for moderator'))
  fireEvent.click(screen.getByRole('button', { name: 'Save moderator permissions' }))
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Moderator permissions updated'))
})
