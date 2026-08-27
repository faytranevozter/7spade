import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
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
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ status: 'ready', environment: 'staging' }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([]), { status: 200 }))
  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Admin sign in' })).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ops@example.com' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(await screen.findByText('Operations overview')).toBeInTheDocument()
  expect(await screen.findByText('STAGING')).toBeInTheDocument()
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(4))
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
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify([]), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ error: 'Sign out failed' }), { status: 500 }))

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

  // invite
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ invitation: { email: 'new@example.com' }, token: 'secret-token-123' }), { status: 201 }))
  // status toggle
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
  // role change
  fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

  render(<App />)

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
})
