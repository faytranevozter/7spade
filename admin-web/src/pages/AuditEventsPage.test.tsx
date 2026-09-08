import '@testing-library/jest-dom/vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { exportAuditEvents, getAuditEvents } from '../api/audit'
import { AuthContext } from '../hooks/useAuth'
import { AuditEventsPage } from './AuditEventsPage'

vi.mock('../api/audit', () => ({
  getAuditEvents: vi.fn(),
  exportAuditEvents: vi.fn(),
}))

const admin = {
  id: 'admin-1',
  email: 'ops@example.com',
  display_name: 'Ops',
  status: 'active',
  permissions: ['audit.read', 'audit.export'],
}

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

function renderPage(initialEntry = '/audit-events') {
  return render(
    <AuthContext.Provider
      value={{
        token: 'token',
        admin,
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
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/audit-events" element={<AuditEventsPage />} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

test('lists audit events and applies URL-backed filters', async () => {
  vi.mocked(getAuditEvents).mockResolvedValue({
    events: [
      {
        id: 'event-1',
        actor_id: 'admin-1',
        action: 'admin.status.update',
        resource_type: 'admin_user',
        resource_id: 'admin-2',
        outcome: 'success',
        occurred_at: '2026-08-20T12:00:00Z',
      },
    ],
    limit: 50,
    offset: 0,
  })

  renderPage()

  expect(await screen.findByText('admin.status.update')).toBeInTheDocument()
  expect(screen.getByText('admin-2')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'View details' })).toHaveAttribute(
    'href',
    expect.stringContaining('/audit-events/event-1'),
  )
  expect(screen.getByRole('option', { name: 'Denied' })).toHaveValue('denied')
  expect(screen.getByRole('option', { name: 'Invalid request' })).toHaveValue(
    'invalid_request',
  )

  fireEvent.change(screen.getByLabelText('Action'), {
    target: { value: 'audit.events.export' },
  })
  fireEvent.change(screen.getByLabelText('Outcome'), {
    target: { value: 'invalid_request' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Apply filters' }))

  await waitFor(() =>
    expect(getAuditEvents).toHaveBeenLastCalledWith(
      'token',
      expect.objectContaining({
        action: 'audit.events.export',
        outcome: 'invalid_request',
      }),
      50,
      0,
    ),
  )
})

test('paginates a full audit page', async () => {
  const events = Array.from({ length: 50 }, (_, index) => ({
    id: `event-${index}`,
    action: `action.${index}`,
    outcome: 'success',
    occurred_at: '2026-08-20T12:00:00Z',
  }))
  vi.mocked(getAuditEvents).mockResolvedValue({ events, limit: 50, offset: 0 })

  renderPage()
  fireEvent.click(await screen.findByRole('button', { name: 'Next' }))

  await waitFor(() =>
    expect(getAuditEvents).toHaveBeenLastCalledWith(
      'token',
      expect.any(Object),
      50,
      50,
    ),
  )
})

test('exports an applied date range', async () => {
  vi.mocked(getAuditEvents).mockResolvedValue({
    events: [],
    limit: 50,
    offset: 0,
  })
  vi.mocked(exportAuditEvents).mockResolvedValue(new Blob(['id,action']))
  const createObjectURL = vi.fn(() => 'blob:audit')
  const revokeObjectURL = vi.fn()
  Object.defineProperty(URL, 'createObjectURL', {
    value: createObjectURL,
    configurable: true,
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    value: revokeObjectURL,
    configurable: true,
  })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
    () => undefined,
  )

  renderPage(
    '/audit-events?outcome=denied&from=2026-08-01T00%3A00&to=2026-08-15T00%3A00',
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Export CSV' }))

  await waitFor(() =>
    expect(exportAuditEvents).toHaveBeenCalledWith(
      'token',
      expect.objectContaining({
        outcome: 'denied',
        from: new Date('2026-08-01T00:00').toISOString(),
        to: new Date('2026-08-15T00:00').toISOString(),
      }),
    ),
  )
  expect(createObjectURL).toHaveBeenCalled()
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:audit')
})
