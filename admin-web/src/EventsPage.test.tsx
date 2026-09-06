import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import { getEvent, transitionEvent } from './api/events'
import { useAuth } from './hooks/useAuth'
import { EventDetailPage } from './pages/EventsPage'
vi.mock('./api/events', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/events')>()),
  getEvent: vi.fn(),
  transitionEvent: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))
const event = {
  id: 'e1',
  slug: 'harvest',
  name: 'Harvest Week',
  summary: 'Gather rewards',
  description: 'Play daily.',
  starts_at: '2026-09-10T00:00:00Z',
  ends_at: '2026-09-17T00:00:00Z',
  reward_config: { daily_login: { enabled: true, xp_per_claim: 125 } },
  state: 'draft' as const,
  revision: 1,
  version: 1,
}
afterEach(cleanup)
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: ['events.read', 'events.manage'] },
  } as ReturnType<typeof useAuth>)
  vi.mocked(getEvent).mockResolvedValue({
    event,
    skin_rewards: [{
      id: 'skin-1',
      skin_type: 'avatar_frame',
      name: 'Harvest Frame',
      description: 'A seasonal frame',
      asset_key: 'skins/harvest.png',
      asset_url: 'https://example.com/harvest.png',
      is_starter: false,
      display_order: 1,
      enabled: true,
      catalog_visible: true,
      unlock_rules_locked: false,
      revisions: [],
      unlock_rules: [{
        id: 'rule-1',
        name: 'Three check-ins',
        rule_type: 'event_check_in_count',
        event_id: 'e1',
        event_check_in_count: 3,
        retroactive: false,
        enabled: true,
      }],
    }],
  })
})
test('previews and schedules an event with a reason', async () => {
  vi.mocked(transitionEvent).mockResolvedValue({
    ...event,
    state: 'scheduled',
    version: 2,
  })
  render(
    <MemoryRouter initialEntries={['/events/e1']}>
      <Routes>
        <Route path="/events/:id" element={<EventDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(
    await screen.findByRole('heading', { name: 'Harvest Week' }),
  ).toBeInTheDocument()
  expect(screen.getByText('Play daily.')).toBeInTheDocument()
  expect(screen.getByText('125 XP')).toBeInTheDocument()
  expect(screen.getByText('Harvest Frame')).toBeInTheDocument()
  expect(screen.getByText(/Check in 3 days/)).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Change reason'), {
    target: { value: 'dates approved' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Schedule' }))
  await waitFor(() =>
    expect(transitionEvent).toHaveBeenCalledWith(
      'token',
      'e1',
      'schedule',
      1,
      'dates approved',
    ),
  )
  expect(await screen.findByText('scheduled')).toBeInTheDocument()
})
test('surfaces optimistic conflicts', async () => {
  vi.mocked(transitionEvent).mockRejectedValue(
    new Error('Event version or lifecycle conflict'),
  )
  render(
    <MemoryRouter initialEntries={['/events/e1']}>
      <Routes>
        <Route path="/events/:id" element={<EventDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
  await screen.findByRole('heading', { name: 'Harvest Week' })
  fireEvent.change(screen.getByLabelText('Change reason'), {
    target: { value: 'publish' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Publish' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('conflict')
})
