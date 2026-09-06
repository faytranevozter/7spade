import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getAuditEvent } from '../api/audit'
import { AuthContext } from '../hooks/useAuth'
import { AuditEventPage } from './AuditEventPage'

vi.mock('../api/audit', () => ({ getAuditEvent: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('shows complete audit event details and returns to filtered results', async () => {
  vi.mocked(getAuditEvent).mockResolvedValue({
    id: 'event-1',
    actor_id: 'admin-1',
    request_id: 'request-1',
    action: 'admin.status.update',
    resource_type: 'admin_user',
    resource_id: 'admin-2',
    reason: 'Access revoked',
    outcome: 'success',
    before_state: { status: 'active' },
    after_state: { status: 'disabled' },
    metadata: { source: 'console' },
    ip_address: '127.0.0.1',
    occurred_at: '2026-08-20T12:00:00Z',
  })

  render(
    <AuthContext.Provider
      value={{
        token: 'token',
        admin: null,
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
      <MemoryRouter
        initialEntries={[
          '/audit-events/event-1?returnTo=%2Faudit-events%3Faction%3Dadmin.status.update',
        ]}
      >
        <Routes>
          <Route path="/audit-events/:id" element={<AuditEventPage />} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>,
  )

  expect(
    await screen.findByRole('heading', { name: 'Audit event' }),
  ).toBeInTheDocument()
  expect(screen.getByText('Access revoked')).toBeInTheDocument()
  expect(screen.getByText('request-1')).toBeInTheDocument()
  expect(screen.getByText('Before state')).toBeInTheDocument()
  expect(
    screen.getByRole('link', { name: /Back to audit log/ }),
  ).toHaveAttribute('href', '/audit-events?action=admin.status.update')
})
