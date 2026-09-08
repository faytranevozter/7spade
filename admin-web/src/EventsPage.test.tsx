import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterAll, afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import { createEvent, getEvent, transitionEvent, updateEvent } from './api/events'
import { useAuth } from './hooks/useAuth'
import { EventCreatePage, EventDetailPage } from './pages/EventsPage'
vi.mock('./api/events', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/events')>()),
  getEvent: vi.fn(),
  createEvent: vi.fn(),
  transitionEvent: vi.fn(),
  updateEvent: vi.fn(),
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
beforeAll(() => {
  vi.stubEnv('TZ', 'America/New_York')
})
afterAll(() => {
  vi.unstubAllEnvs()
})
afterEach(cleanup)
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: ['events.read', 'events.manage'] },
  } as ReturnType<typeof useAuth>)
  vi.mocked(getEvent).mockResolvedValue({
    event,
    skin_rewards: [
      {
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
        unlock_rules: [
          {
            id: 'rule-1',
            name: 'Three check-ins',
            rule_type: 'event_check_in_count',
            event_id: 'e1',
            event_check_in_count: 3,
            retroactive: false,
            enabled: true,
          },
        ],
      },
    ],
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

test('edits daily login rewards without exposing JSON', async () => {
  const configuredEvent = {
    ...event,
    reward_config: {
      daily_login: { enabled: true, xp_per_claim: 125 },
      preserved_setting: 'keep-me',
    },
  }
  vi.mocked(getEvent).mockResolvedValue({
    event: configuredEvent,
    skin_rewards: [],
  })
  vi.mocked(updateEvent).mockResolvedValue({
    ...configuredEvent,
    reward_config: {
      ...configuredEvent.reward_config,
      daily_login: { enabled: false, xp_per_claim: 250 },
    },
  })

  render(
    <MemoryRouter initialEntries={['/events/e1/edit']}>
      <Routes>
        <Route path="/events/:id/edit" element={<EventCreatePage />} />
        <Route path="/events/:id" element={<div>Saved event</div>} />
      </Routes>
    </MemoryRouter>,
  )

  const toggle = await screen.findByRole('checkbox', {
    name: /Enable daily login rewards/i,
  })
  const xp = screen.getByLabelText('XP per daily claim')
  expect(screen.queryByText('JSON configuration')).not.toBeInTheDocument()
  fireEvent.change(xp, { target: { value: '250' } })
  fireEvent.click(toggle)
  fireEvent.change(screen.getByLabelText('Change reason'), {
    target: { value: 'adjust campaign rewards' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save new revision' }))

  await waitFor(() => expect(updateEvent).toHaveBeenCalled())
  expect(vi.mocked(updateEvent).mock.calls[0][2]).toMatchObject({
    reward_config: {
      daily_login: { enabled: false, xp_per_claim: 250 },
      preserved_setting: 'keep-me',
    },
    reason: 'adjust campaign rewards',
  })
})

test('loads UTC event times as local values and converts edits on update', async () => {
  vi.mocked(updateEvent).mockResolvedValue(event)

  render(
    <MemoryRouter initialEntries={['/events/e1/edit']}>
      <Routes>
        <Route path="/events/:id/edit" element={<EventCreatePage />} />
        <Route path="/events/:id" element={<div>Saved event</div>} />
      </Routes>
    </MemoryRouter>,
  )

  const startsAt = await screen.findByLabelText('Starts at')
  const endsAt = screen.getByLabelText('Ends at')
  expect(startsAt).toHaveValue('2026-09-09T20:00')
  expect(endsAt).toHaveValue('2026-09-16T20:00')

  fireEvent.change(startsAt, { target: { value: '' } })
  expect(startsAt).toHaveValue('')
  fireEvent.change(startsAt, { target: { value: '2026-11-01T01:30' } })
  fireEvent.change(startsAt, { target: { value: '2026-11-01T02:30' } })
  expect(startsAt).toHaveValue('2026-11-01T02:30')
  fireEvent.change(screen.getByLabelText('Change reason'), {
    target: { value: 'adjust local schedule' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save new revision' }))

  await waitFor(() => expect(updateEvent).toHaveBeenCalled())
  expect(vi.mocked(updateEvent).mock.calls[0][2]).toMatchObject({
    starts_at: '2026-11-01T07:30:00.000Z',
    ends_at: '2026-09-17T00:00:00.000Z',
  })
})

test('keeps create times local until submitting valid nonempty values', async () => {
  vi.mocked(createEvent).mockResolvedValue(event)

  render(
    <MemoryRouter initialEntries={['/events/new']}>
      <Routes>
        <Route path="/events/new" element={<EventCreatePage />} />
        <Route path="/events/:id" element={<div>Saved event</div>} />
      </Routes>
    </MemoryRouter>,
  )

  const startsAt = screen.getByLabelText('Starts at')
  fireEvent.change(startsAt, { target: { value: '2026-03-08T01:30' } })
  fireEvent.change(screen.getByLabelText('Ends at'), {
    target: { value: '2026-03-08T03:30' },
  })
  expect(startsAt).toHaveValue('2026-03-08T01:30')
  fireEvent.change(screen.getByLabelText('Change reason'), {
    target: { value: 'create local campaign' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Create draft' }))

  await waitFor(() => expect(createEvent).toHaveBeenCalled())
  expect(vi.mocked(createEvent).mock.calls[0][1]).toMatchObject({
    starts_at: '2026-03-08T06:30:00.000Z',
    ends_at: '2026-03-08T07:30:00.000Z',
  })
})
