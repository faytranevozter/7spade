import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { VerifyEmailBanner } from './VerifyEmailBanner'
import { getMe, postResendVerification } from '../api/auth'
import { AuthContext, type UseAuthReturn } from '../hooks/useAuth'

vi.mock('../api/auth', () => ({
  getMe: vi.fn(),
  postResendVerification: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(getMe).mockResolvedValue({
    user_id: 'me',
    username: 'me',
    display_name: 'Me',
    avatar_url: null,
    created_at: null,
    is_guest: false,
    email_verified: false,
    providers: [],
  })
  vi.mocked(postResendVerification).mockResolvedValue(undefined)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function tokenWithClaims(claims: Record<string, unknown>) {
  const payload = btoa(JSON.stringify(claims)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
  return `header.${payload}.signature`
}

function renderBanner(authOverrides: Partial<UseAuthReturn> = {}) {
  const value: UseAuthReturn = {
    token: tokenWithClaims({ sub: 'me', is_guest: false }),
    isAuthenticated: true,
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
    ...authOverrides,
  }

  return render(
    <AuthContext.Provider value={value}>
      <VerifyEmailBanner />
    </AuthContext.Provider>,
  )
}

test('shows the verify prompt for registered unverified users and resends email', async () => {
  renderBanner()

  expect(await screen.findByText('Please verify your email address to secure your account.')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Resend email' }))

  await waitFor(() => {
    expect(postResendVerification).toHaveBeenCalledWith(expect.any(String))
  })
  expect(await screen.findByText('Verification email sent. Check your inbox.')).toBeInTheDocument()
})

test('dismisses the banner', async () => {
  renderBanner()

  expect(await screen.findByText(/Please verify your email/i)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))

  expect(screen.queryByText(/Please verify your email/i)).not.toBeInTheDocument()
})

test('does not render for guests, signed-out users, or verified users', async () => {
  const guestToken = tokenWithClaims({ sub: 'guest', is_guest: true })
  const { rerender } = renderBanner({ token: guestToken })

  expect(screen.queryByText(/Please verify your email/i)).not.toBeInTheDocument()
  expect(getMe).not.toHaveBeenCalled()

  rerender(
    <AuthContext.Provider
      value={{
        token: null,
        isAuthenticated: false,
        isLoading: false,
        login: vi.fn(),
        logout: vi.fn(),
      }}
    >
      <VerifyEmailBanner />
    </AuthContext.Provider>,
  )
  expect(screen.queryByText(/Please verify your email/i)).not.toBeInTheDocument()

  vi.mocked(getMe).mockResolvedValueOnce({
    user_id: 'me',
    username: 'me',
    display_name: 'Me',
    avatar_url: null,
    created_at: null,
    is_guest: false,
    email_verified: true,
    providers: [],
  })
  rerender(
    <AuthContext.Provider
      value={{
        token: tokenWithClaims({ sub: 'me', is_guest: false }),
        isAuthenticated: true,
        isLoading: false,
        login: vi.fn(),
        logout: vi.fn(),
      }}
    >
      <VerifyEmailBanner />
    </AuthContext.Provider>,
  )

  await waitFor(() => expect(getMe).toHaveBeenCalled())
  expect(screen.queryByText(/Please verify your email/i)).not.toBeInTheDocument()
})
