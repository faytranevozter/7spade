import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, act } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { ForgotPasswordPage } from './ForgotPasswordPage'
import { ResetPasswordPage } from './ResetPasswordPage'
import { VerifyEmailPage } from './VerifyEmailPage'
import { postForgotPassword, postResetPassword, postVerifyEmail } from '../api/auth'

vi.mock('../api/auth', () => ({
  AuthApiError: class AuthApiError extends Error {
    statusCode: number
    constructor(message: string, statusCode: number) {
      super(message)
      this.statusCode = statusCode
    }
  },
  postForgotPassword: vi.fn(),
  postResetPassword: vi.fn(),
  postVerifyEmail: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(postForgotPassword).mockResolvedValue(undefined)
  vi.mocked(postResetPassword).mockResolvedValue(undefined)
  vi.mocked(postVerifyEmail).mockResolvedValue(undefined)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  vi.useRealTimers()
})

function renderAt(path: string, element: React.ReactElement) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/forgot-password" element={element} />
        <Route path="/reset-password" element={element} />
        <Route path="/verify-email" element={element} />
        <Route path="/auth" element={<div>Sign in landing</div>} />
        <Route path="/lobby" element={<div>Lobby landing</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

test('forgot-password submits the email and shows a confirmation', async () => {
  renderAt('/forgot-password', <ForgotPasswordPage />)
  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'a@b.com' } })
  fireEvent.click(screen.getByRole('button', { name: /Send reset link/i }))

  await waitFor(() => {
    expect(postForgotPassword).toHaveBeenCalledWith('a@b.com')
  })
  expect(await screen.findByText(/Check your inbox/i)).toBeInTheDocument()
  expect(screen.getByText('a@b.com')).toBeInTheDocument()
})

test('forgot-password rejects invalid email format before calling the API', () => {
  renderAt('/forgot-password', <ForgotPasswordPage />)
  // Passes HTML type=email in jsdom but fails our host.tld shape check.
  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'user@localhost' } })
  fireEvent.submit(screen.getByRole('button', { name: /Send reset link/i }).closest('form')!)

  expect(screen.getByRole('alert')).toHaveTextContent(/valid email/i)
  expect(postForgotPassword).not.toHaveBeenCalled()
})

test('forgot-password resend respects cooldown then calls the API again', async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  renderAt('/forgot-password', <ForgotPasswordPage />)

  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'a@b.com' } })
  fireEvent.click(screen.getByRole('button', { name: /Send reset link/i }))

  await waitFor(() => {
    expect(postForgotPassword).toHaveBeenCalledTimes(1)
  })

  expect(screen.getByRole('button', { name: /Resend in \d+s/i })).toBeDisabled()

  await act(async () => {
    await vi.advanceTimersByTimeAsync(60_000)
  })

  const resend = screen.getByRole('button', { name: /Resend reset link/i })
  expect(resend).not.toBeDisabled()
  fireEvent.click(resend)

  await waitFor(() => {
    expect(postForgotPassword).toHaveBeenCalledTimes(2)
  })
  expect(postForgotPassword).toHaveBeenLastCalledWith('a@b.com')
})

test('forgot-password try again returns to the form', async () => {
  renderAt('/forgot-password', <ForgotPasswordPage />)
  fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: 'a@b.com' } })
  fireEvent.click(screen.getByRole('button', { name: /Send reset link/i }))

  expect(await screen.findByText(/Check your inbox/i)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /Wrong email/i }))

  expect(screen.getByRole('heading', { name: /Reset your password/i })).toBeInTheDocument()
  expect(screen.getByLabelText(/Email/i)).toHaveValue('a@b.com')
})

test('reset-password rejects mismatched passwords before calling the API', () => {
  renderAt('/reset-password?token=tok123', <ResetPasswordPage />)
  fireEvent.change(screen.getByLabelText(/New password/i), { target: { value: 'longenough1' } })
  fireEvent.change(screen.getByLabelText(/Confirm password/i), { target: { value: 'different123' } })
  fireEvent.click(screen.getByRole('button', { name: /Update password/i }))

  expect(screen.getByRole('alert')).toHaveTextContent(/do not match/i)
  expect(postResetPassword).not.toHaveBeenCalled()
})

test('reset-password submits token + new password', async () => {
  renderAt('/reset-password?token=tok123', <ResetPasswordPage />)
  fireEvent.change(screen.getByLabelText(/New password/i), { target: { value: 'longenough1' } })
  fireEvent.change(screen.getByLabelText(/Confirm password/i), { target: { value: 'longenough1' } })
  fireEvent.click(screen.getByRole('button', { name: /Update password/i }))

  await waitFor(() => {
    expect(postResetPassword).toHaveBeenCalledWith('tok123', 'longenough1')
  })
  expect(await screen.findByText(/Password updated/i)).toBeInTheDocument()
})

test('reset-password without a token shows an invalid-link message', () => {
  renderAt('/reset-password', <ResetPasswordPage />)
  expect(screen.getByText(/Invalid link/i)).toBeInTheDocument()
  expect(postResetPassword).not.toHaveBeenCalled()
})

test('verify-email auto-verifies the token on mount', async () => {
  renderAt('/verify-email?token=vtok', <VerifyEmailPage />)
  await waitFor(() => {
    expect(postVerifyEmail).toHaveBeenCalledWith('vtok')
  })
  expect(await screen.findByText(/Email verified/i)).toBeInTheDocument()
})
