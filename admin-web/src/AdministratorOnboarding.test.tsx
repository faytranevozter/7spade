import '@testing-library/jest-dom/vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  window.location.hash = ''
  vi.restoreAllMocks()
})

test('invitee validates an invitation and creates an administrator account', async () => {
  window.location.hash = '#/accept-invitation?token=invite-token'
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockImplementation(async (input, init) => {
      const url = String(input)
      if (
        url.endsWith('/auth/invitations/inspect') &&
        init?.method === 'POST'
      ) {
        return new Response(
          JSON.stringify({
            id: 'invite-1',
            email: 'new@example.com',
            role_id: 'role-viewer',
            role_name: 'viewer',
            expires_at: '2027-01-01T00:00:00Z',
            created_at: '2026-09-07T00:00:00Z',
          }),
          { status: 200 },
        )
      }
      if (url.endsWith('/auth/invitations/accept') && init?.method === 'POST') {
        return new Response(
          JSON.stringify({ id: 'admin-2', email: 'new@example.com' }),
          { status: 200 },
        )
      }
      throw new Error(`Unexpected request: ${url}`)
    })

  render(<App />)
  expect(await screen.findByText('new@example.com')).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Display name'), {
    target: { value: 'New Operator' },
  })
  fireEvent.change(screen.getByLabelText('Password'), {
    target: { value: 'secure-password' },
  })
  fireEvent.change(screen.getByLabelText('Confirm password'), {
    target: { value: 'secure-password' },
  })
  fireEvent.click(
    screen.getByRole('button', { name: 'Create administrator account' }),
  )

  expect(
    await screen.findByText('Administrator account created.'),
  ).toBeInTheDocument()
  const request = fetchMock.mock.calls.find(([url]) =>
    String(url).endsWith('/auth/invitations/accept'),
  )
  expect(request?.[1]?.body).toBe(
    JSON.stringify({
      token: 'invite-token',
      display_name: 'New Operator',
      password: 'secure-password',
    }),
  )
})

test('administrator enrolls MFA and receives recovery codes', async () => {
  window.location.hash = '#/security'
  const admin = {
    id: 'admin-1',
    email: 'ops@example.com',
    display_name: 'Operator',
    status: 'active',
    permissions: [],
    mfa_enrolled: false,
  }
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh'))
      return new Response(JSON.stringify({ access_token: 'access', admin }), {
        status: 200,
      })
    if (url.endsWith('/auth/mfa/enroll'))
      return new Response(
        JSON.stringify({
          secret: 'TOTPSECRET',
          uri: 'otpauth://totp/SevenSpade',
        }),
        { status: 200 },
      )
    if (url.endsWith('/auth/mfa/confirm'))
      return new Response(
        JSON.stringify({ recovery_codes: ['code-one', 'code-two'] }),
        { status: 200 },
      )
    throw new Error(`Unexpected request: ${url}`)
  })

  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: 'Set up MFA' }))
  expect(await screen.findByText('TOTPSECRET')).toBeInTheDocument()
  expect(
    screen.getByText('Scan to add Seven Spade administrator MFA'),
  ).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Six-digit verification code'), {
    target: { value: '123456' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Verify and enable' }))
  await waitFor(() => expect(screen.getByText(/code-one/)).toBeInTheDocument())
})
