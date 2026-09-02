import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getEvents } from './api/events'
import { useAuth } from './hooks/useAuth'
import { EventsPage } from './pages/EventsPage'

vi.mock('./api/events', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/events')>()),
  getEvents: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))
afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('paginates event records', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: [] },
  } as unknown as ReturnType<typeof useAuth>)
  vi.mocked(getEvents).mockResolvedValue({
    events: Array.from({ length: 13 }, (_, index) => ({
      id: `event-${index + 1}`,
      slug: `event-${index + 1}`,
      name: `Event ${index + 1}`,
      summary: '',
      description: '',
      starts_at: '2026-09-10T00:00:00Z',
      ends_at: '2026-09-17T00:00:00Z',
      reward_config: {},
      state: 'draft' as const,
      revision: 1,
      version: 1,
    })),
  })
  render(
    <MemoryRouter>
      <EventsPage />
    </MemoryRouter>,
  )
  expect(
    await screen.findByRole('heading', { name: 'Event 1' }),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Event 13' }),
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  expect(
    await screen.findByRole('heading', { name: 'Event 13' }),
  ).toBeInTheDocument()
})
