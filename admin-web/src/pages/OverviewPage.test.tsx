import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { getSessions } from '../api/auth'
import { getAuditEvents } from '../api/audit'
import { getDashboard } from '../api/dashboard'
import { getEvents } from '../api/events'
import { AuthContext } from '../hooks/useAuth'
import { OverviewPage } from './OverviewPage'

vi.mock('../api/auth', () => ({
  getSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
}))
vi.mock('../api/audit', () => ({ getAuditEvents: vi.fn() }))
vi.mock('../api/dashboard', () => ({ getDashboard: vi.fn() }))
vi.mock('../api/events', () => ({ getEvents: vi.fn() }))

const dashboard = {
  status: 'ready',
  environment: 'development',
  windows: {
    day: { from: '2026-09-07T00:00:00Z', to: '2026-09-08T00:00:00Z' },
    month: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' },
  },
  current: { players: 12, rooms: 4, games: 2 },
  daily: {
    registrations: 3,
    players: 18,
    rooms: 6,
    games_started: 9,
    games_completed: 7,
    games_abandoned: 2,
    average_game_duration_seconds: 185,
  },
  monthly: {
    registrations: 42,
    players: 120,
    rooms: 65,
    games_started: 90,
    games_completed: 81,
    games_abandoned: 9,
    average_game_duration_seconds: 240,
  },
  services: { api: { status: 'ok' }, ws: { status: 'ok' } },
  links: [],
}

beforeEach(() => {
  vi.mocked(getDashboard).mockResolvedValue(dashboard)
  vi.mocked(getSessions).mockResolvedValue([])
  vi.mocked(getEvents).mockResolvedValue({ events: [] })
  vi.mocked(getAuditEvents).mockResolvedValue({ events: [], limit: 6, offset: 0 })
})

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

function renderOverview(mfaEnrolled = true) {
  render(
    <MemoryRouter>
      <AuthContext.Provider
        value={{
          token: 'token',
          admin: {
            id: 'admin-1',
            email: 'ops@example.com',
            display_name: 'Operator',
            status: 'active',
            permissions: [
              'dashboard.read',
              'events.read',
              'audit.read',
              'users.read',
              'rooms.read',
              'games.read',
            ],
            mfa_enrolled: mfaEnrolled,
          },
          challengeToken: '',
          isLoading: false,
          error: '',
          signIn: vi.fn(),
          completeMFA: vi.fn(),
          signOut: vi.fn(),
          refreshSession: vi.fn(),
          expireSession: vi.fn(),
        }}
      >
        <OverviewPage />
      </AuthContext.Provider>
    </MemoryRouter>,
  )
}

test('shows healthy operations and real snapshot activity', async () => {
  renderOverview()

  expect(await screen.findByText('No detected operational issues')).toBeInTheDocument()
  expect(screen.getByText('12')).toBeInTheDocument()
  expect(screen.getByText('Connected players')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Investigate users/ })).toHaveAttribute('href', '/users')
})

test('surfaces degraded services and missing MFA as attention items', async () => {
  vi.mocked(getDashboard).mockResolvedValue({
    ...dashboard,
    services: { api: { status: 'degraded' }, ws: { status: 'unreachable' } },
  })
  renderOverview(false)

  expect(await screen.findByText('3 items need attention')).toBeInTheDocument()
  expect(screen.getByText('API Degraded')).toBeInTheDocument()
  expect(screen.getByText('WebSocket Unreachable')).toBeInTheDocument()
  expect(screen.getByText('MFA is not enrolled')).toBeInTheDocument()
})

test('switches between today and monthly activity', async () => {
  renderOverview()
  expect(await screen.findByText('3m 5s')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'This month' }))

  expect(screen.getByText('4m 0s')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'This month' })).toHaveAttribute('aria-pressed', 'true')
})

test('keeps the dashboard useful when optional sections fail', async () => {
  vi.mocked(getEvents).mockRejectedValue(new Error('events unavailable'))
  vi.mocked(getAuditEvents).mockRejectedValue(new Error('audit unavailable'))
  renderOverview()

  expect(await screen.findByText('12')).toBeInTheDocument()
  await waitFor(() => expect(screen.getByText('Event schedule could not be loaded.')).toBeInTheDocument())
  expect(screen.getByText('Recent audit activity could not be loaded.')).toBeInTheDocument()
})
