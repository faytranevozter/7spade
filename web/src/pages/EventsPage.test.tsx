import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getEvents } from '../api/events'
import { EventsPage } from './EventsPage'

vi.mock('../api/events', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/events')>()
  return { ...actual, getEvents: vi.fn() }
})

const mockedGetEvents = vi.mocked(getEvents)

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('lists events and filters them by status', async () => {
  mockedGetEvents.mockResolvedValue({ events: [
    event('live', 'Live Event', 'active'),
    event('next', 'Next Event', 'upcoming'),
    event('past', 'Past Event', 'ended'),
  ] })

  render(<MemoryRouter><EventsPage /></MemoryRouter>)

  expect(screen.getByLabelText('Loading events')).toBeInTheDocument()
  const liveHeading = await screen.findByRole('heading', { name: 'Live Event' })
  expect(liveHeading).toBeInTheDocument()
  expect(within(liveHeading.closest('article')!).getByRole('link', { name: /View event/i })).toHaveAttribute('href', '/events/live')

  fireEvent.click(screen.getByRole('button', { name: 'Upcoming' }))
  expect(screen.getByRole('heading', { name: 'Next Event' })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: 'Live Event' })).not.toBeInTheDocument()
})

test('shows empty and error states', async () => {
  mockedGetEvents.mockResolvedValueOnce({ events: [] })
  const { unmount } = render(<MemoryRouter><EventsPage /></MemoryRouter>)
  expect(await screen.findByRole('heading', { name: /No events found/i })).toBeInTheDocument()
  unmount()

  mockedGetEvents.mockRejectedValueOnce(new Error('offline'))
  render(<MemoryRouter><EventsPage /></MemoryRouter>)
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Failed to load events'))
})

function event(slug: string, name: string, status: 'active' | 'upcoming' | 'ended') {
  return {
    id: slug,
    slug,
    name,
    summary: `${name} summary`,
    starts_at: '2026-08-20T00:00:00Z',
    ends_at: '2026-09-03T00:00:00Z',
    app_timezone: 'UTC',
    status,
    server_time: '2026-08-27T00:00:00Z',
    reward_count: 2,
  }
}
