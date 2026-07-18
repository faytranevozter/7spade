import { type FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router'
import { Button } from '../components/Button'
import {
  AuthCardShell,
  authErrorClassName,
  authFieldClassName,
  authLabelClassName,
} from '../components/AuthCardShell'
import { postForgotPassword, AuthApiError } from '../api/auth'

const RESEND_COOLDOWN_SECONDS = 60

/** Lightweight format check before we hit the API (enumeration-safe response still applies). */
function isValidEmailFormat(value: string): boolean {
  // Practical shape check — not full RFC 5322. Rejects spaces and requires user@host.tld.
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [submittedEmail, setSubmittedEmail] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [cooldown, setCooldown] = useState(0)

  useEffect(() => {
    if (cooldown <= 0) return
    const id = window.setInterval(() => {
      setCooldown((s) => (s <= 1 ? 0 : s - 1))
    }, 1000)
    return () => window.clearInterval(id)
  }, [cooldown])

  const sendReset = async (targetEmail: string) => {
    setError(null)
    if (!isValidEmailFormat(targetEmail)) {
      setError('Enter a valid email address')
      return
    }
    setIsLoading(true)
    try {
      await postForgotPassword(targetEmail)
      setSubmittedEmail(targetEmail)
      setCooldown(RESEND_COOLDOWN_SECONDS)
    } catch (err) {
      setError(err instanceof AuthApiError ? err.message : 'Something went wrong. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    await sendReset(email.trim().toLowerCase())
  }

  const handleResend = async () => {
    if (!submittedEmail || cooldown > 0 || isLoading) return
    await sendReset(submittedEmail)
  }

  const handleTryAgain = () => {
    setSubmittedEmail(null)
    setError(null)
    setCooldown(0)
  }

  if (submittedEmail) {
    return (
      <AuthCardShell
        title="Check your inbox"
        subtitle="If an account exists for that email, we've sent a reset link."
        footer={
          <div className="space-y-3 text-center text-sm text-spade-gray-3">
            <p>
              <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
                Back to sign in
              </Link>
            </p>
            <p>
              <button
                type="button"
                onClick={handleTryAgain}
                className="font-medium text-spade-gold hover:text-spade-gold-light"
              >
                Wrong email? Try again
              </button>
            </p>
          </div>
        }
      >
        <div className="grid gap-4">
          <p className="text-sm text-spade-gray-2">
            We sent a link to{' '}
            <span className="break-all font-medium text-spade-cream">{submittedEmail}</span>. The link
            expires in 15 minutes. If you don&apos;t see it, check spam or wait a few minutes and try
            again.
          </p>

          {error ? (
            <div role="alert" className={authErrorClassName}>
              {error}
            </div>
          ) : null}

          <Button
            type="button"
            variant="secondary"
            className="w-full py-3"
            disabled={isLoading || cooldown > 0}
            onClick={handleResend}
          >
            {isLoading
              ? 'Sending…'
              : cooldown > 0
                ? `Resend in ${cooldown}s`
                : 'Resend reset link'}
          </Button>
        </div>
      </AuthCardShell>
    )
  }

  return (
    <AuthCardShell
      title="Reset your password"
      subtitle="Enter your email and we'll send you a reset link."
      footer={
        <p className="text-center text-sm text-spade-gray-3">
          <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
            Back to sign in
          </Link>
        </p>
      }
    >
      <form onSubmit={handleSubmit} className="grid gap-4">
        <label className={authLabelClassName}>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            required
            disabled={isLoading}
            autoComplete="email"
            className={authFieldClassName}
          />
        </label>

        {error ? (
          <div role="alert" className={authErrorClassName}>
            {error}
          </div>
        ) : null}

        <Button type="submit" className="w-full py-3" disabled={isLoading || !email.trim()}>
          {isLoading ? 'Sending…' : 'Send reset link'}
        </Button>
      </form>
    </AuthCardShell>
  )
}
