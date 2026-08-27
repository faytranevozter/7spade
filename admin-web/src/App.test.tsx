import '@testing-library/jest-dom/vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import App from './App'

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
