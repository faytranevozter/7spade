import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { useAuth } from './hooks/useAuth'
import { Notice } from './components/Feedback'
import { SkinDetailPage } from './pages/SkinDetailPage'
import { SkinsPage } from './pages/SkinsPage'
import {
  EventsPage,
  EventDetailPage,
  EventCreatePage,
} from './pages/EventsPage'
import {
  AchievementsPage,
  AchievementDetailPage,
} from './pages/AchievementsPage'
import { SettingsPage } from './pages/SettingsPage'
import { AdministratorsPage } from './pages/AdministratorsPage'
import { RolesPage } from './pages/RolesPage'
import { RoomDetailPage } from './pages/RoomDetailPage'
import { GameDetailPage } from './pages/GameDetailPage'
import { UserDetailPage } from './pages/UserDetailPage'

vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const pages = [
  SkinDetailPage,
  SkinsPage,
  EventsPage,
  EventDetailPage,
  EventCreatePage,
  AchievementsPage,
  AchievementDetailPage,
  SettingsPage,
  AdministratorsPage,
  RolesPage,
  RoomDetailPage,
  GameDetailPage,
  UserDetailPage,
]

test.each(pages)(
  '%s uses the shared red alert for authentication failures',
  async (Page) => {
    vi.mocked(useAuth).mockReturnValue({
      token: 'test-token',
      admin: { permissions: ['admins.read'] },
    } as ReturnType<typeof useAuth>)
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: 'Authentication required' }),
      }),
    )
    render(
      <MemoryRouter initialEntries={['/record/test-id']}>
        <Routes>
          <Route path="/record/:id" element={<Page />} />
        </Routes>
      </MemoryRouter>,
    )
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Authentication required')
    expect(alert).toHaveClass(
      'border-admin-danger-border',
      'bg-admin-danger-bg',
      'text-admin-danger',
      'border-l-[3px]',
      'rounded-r-md',
    )
    expect(alert).not.toHaveClass(
      'text-admin-success',
      'text-admin-warning',
      'border-dashed',
    )
    expect(screen.queryByText(/Loading .*details/)).not.toBeInTheDocument()
  },
)

test.each([
  SkinDetailPage,
  EventDetailPage,
  RoomDetailPage,
  GameDetailPage,
  UserDetailPage,
])('%s keeps loading separate from errors', (Page) => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'test-token',
    admin: { permissions: [] },
  } as unknown as ReturnType<typeof useAuth>)
  vi.stubGlobal(
    'fetch',
    vi.fn(() => new Promise(() => {})),
  )
  render(
    <MemoryRouter initialEntries={['/record/test-id']}>
      <Routes>
        <Route path="/record/:id" element={<Page />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(screen.getByRole('status')).toHaveTextContent('Loading')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('success notices do not inherit error or warning colors', () => {
  render(<Notice variant="success">Saved</Notice>)
  expect(screen.getByRole('status')).toHaveClass(
    'text-admin-success',
    'bg-admin-success-bg',
    'border-admin-success-border',
  )
  expect(screen.getByRole('status')).not.toHaveClass(
    'text-admin-danger',
    'text-admin-warning',
    'bg-admin-accent/9',
  )
})
