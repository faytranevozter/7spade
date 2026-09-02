import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getEvents } from './api/events'
import { getAchievements, getSkins } from './api/skins'
import { useAuth } from './hooks/useAuth'
import { SkinDetailPage } from './pages/SkinDetailPage'

vi.mock('./api/events', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/events')>()),
  getEvents: vi.fn(),
}))
vi.mock('./api/skins', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/skins')>()),
  getAchievements: vi.fn(),
  getSkins: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('selects an event for an event check-in unlock rule', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: ['skins.manage'] },
  } as ReturnType<typeof useAuth>)
  vi.mocked(getSkins).mockResolvedValue({
    skins: [
      {
        id: 'skin-1',
        skin_type: 'avatar_frame',
        name: 'Aurora Frame',
        description: '',
        asset_key: '',
        asset_url: '',
        is_starter: false,
        display_order: 0,
        enabled: true,
        catalog_visible: true,
        unlock_rules_locked: false,
        unlock_rules: [],
        revisions: [],
      },
    ],
  })
  vi.mocked(getAchievements).mockResolvedValue({ achievements: [] })
  vi.mocked(getEvents).mockResolvedValue({
    events: [
      {
        id: 'event-1',
        slug: 'spring-festival',
        name: 'Spring Festival',
        summary: '',
        description: '',
        starts_at: '2026-03-01T00:00:00Z',
        ends_at: '2026-03-31T00:00:00Z',
        reward_config: {},
        state: 'published',
        revision: 1,
        version: 1,
      },
    ],
  })

  render(
    <MemoryRouter initialEntries={['/skins/skin-1']}>
      <Routes>
        <Route path="/skins/:id" element={<SkinDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )

  fireEvent.click(
    await screen.findByRole('button', { name: 'Add unlock rule' }),
  )
  fireEvent.change(screen.getByLabelText('Rule type'), {
    target: { value: 'event_check_in_count' },
  })
  const eventSelect = screen.getByLabelText('Event')
  expect(
    screen.getByRole('option', { name: 'Spring Festival' }),
  ).toBeInTheDocument()
  fireEvent.change(eventSelect, { target: { value: 'event-1' } })
  expect(eventSelect).toHaveValue('event-1')
})

test('scopes a game condition to an event', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: ['skins.manage'] },
  } as ReturnType<typeof useAuth>)
  vi.mocked(getSkins).mockResolvedValue({
    skins: [{
      id: 'skin-1', skin_type: 'avatar_frame', name: 'Aurora Frame',
      description: '', asset_key: '', asset_url: '', is_starter: false,
      display_order: 0, enabled: true, catalog_visible: true,
      unlock_rules_locked: false, unlock_rules: [], revisions: [],
    }],
  })
  vi.mocked(getAchievements).mockResolvedValue({ achievements: [] })
  vi.mocked(getEvents).mockResolvedValue({
    events: [{
      id: 'event-1', slug: 'spring-festival', name: 'Spring Festival',
      summary: '', description: '', starts_at: '2026-03-01T00:00:00Z',
      ends_at: '2026-03-31T00:00:00Z', reward_config: {}, state: 'published',
      revision: 1, version: 1,
    }],
  })

  render(
    <MemoryRouter initialEntries={['/skins/skin-1']}>
      <Routes><Route path="/skins/:id" element={<SkinDetailPage />} /></Routes>
    </MemoryRouter>,
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Add unlock rule' }))
  fireEvent.change(screen.getByLabelText('Rule type'), { target: { value: 'game_condition' } })
  fireEvent.change(screen.getByLabelText('Scope'), { target: { value: 'event' } })

  expect(screen.getByLabelText('Event')).toHaveValue('event-1')
})

test('allows a game condition to target a draft event', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: ['skins.manage'] },
  } as ReturnType<typeof useAuth>)
  vi.mocked(getSkins).mockResolvedValue({
    skins: [{
      id: 'skin-1', skin_type: 'avatar_frame', name: 'Draft Frame',
      description: '', asset_key: '', asset_url: '', is_starter: false,
      display_order: 0, enabled: true, catalog_visible: true,
      unlock_rules_locked: false, unlock_rules: [], revisions: [],
    }],
  })
  vi.mocked(getAchievements).mockResolvedValue({ achievements: [] })
  vi.mocked(getEvents).mockResolvedValue({
    events: [{
      id: 'event-draft', slug: 'next-event', name: 'Next Event',
      summary: '', description: '', starts_at: '2026-10-01T00:00:00Z',
      ends_at: '2026-10-31T00:00:00Z', reward_config: {}, state: 'draft',
      revision: 1, version: 1,
    }],
  })

  render(
    <MemoryRouter initialEntries={['/skins/skin-1']}>
      <Routes><Route path="/skins/:id" element={<SkinDetailPage />} /></Routes>
    </MemoryRouter>,
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Add unlock rule' }))
  fireEvent.change(screen.getByLabelText('Rule type'), { target: { value: 'game_condition' } })

  expect(screen.getByRole('option', { name: 'Event' })).not.toBeDisabled()
})
