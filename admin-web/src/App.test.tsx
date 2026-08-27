import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

test('administrator signs in and sees the protected dashboard', async () => {
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin: { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['dashboard.read'] } }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ status: 'ready', environment: 'staging' }), { status: 200 }))
  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Admin sign in' })).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'ops@example.com' } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'secret' } })
  fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
  expect(await screen.findByText('Operations overview')).toBeInTheDocument()
  expect(await screen.findByText('STAGING')).toBeInTheDocument()
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3))
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
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'expired', admin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ error: 'Authentication required' }), { status: 401 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'fresh', admin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ status: 'ready', environment: 'production' }), { status: 200 }))

  render(<App />)

  expect(await screen.findByText('PRODUCTION')).toBeInTheDocument()
  expect(fetchMock.mock.calls[3]?.[1]?.headers).toEqual({ Authorization: 'Bearer fresh' })
})

test('failed logout clears local access and reports the failure', async () => {
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: [] }
  const fetchMock = vi.spyOn(globalThis, 'fetch')
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 }))
  fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ error: 'Sign out failed' }), { status: 500 }))

  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Sign out' }))

  expect(await screen.findByRole('heading', { name: 'Admin sign in' })).toBeInTheDocument()
  expect(screen.getByRole('alert')).toHaveTextContent('Sign out failed')
})
