import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import {
  AuthCardShell,
  authErrorClassName,
} from '../components/AuthCardShell'
import { postVerifyEmail, AuthApiError } from '../api/auth'

type Status = 'verifying' | 'success' | 'error'

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const [status, setStatus] = useState<Status>(token ? 'verifying' : 'error')
  const [error, setError] = useState<string | null>(
    token ? null : 'This verification link is missing its token.',
  )
  // Guard against React StrictMode double-invoke consuming the single-use token twice.
  const startedRef = useRef(false)

  useEffect(() => {
    if (!token || startedRef.current) return
    startedRef.current = true
    postVerifyEmail(token)
      .then(() => setStatus('success'))
      .catch((err) => {
        setStatus('error')
        setError(
          err instanceof AuthApiError
            ? err.message
            : 'Verification failed. The link may have expired.',
        )
      })
  }, [token])

  if (status === 'verifying') {
    return (
      <AuthCardShell title="Verifying…" subtitle="Confirming your email address.">
        <p className="text-center text-sm text-spade-gray-2">This usually only takes a moment.</p>
      </AuthCardShell>
    )
  }

  if (status === 'success') {
    return (
      <AuthCardShell
        title="Email verified"
        subtitle="Thanks! Your email address is now verified."
        footer={
          <p className="text-center text-sm text-spade-gray-3">
            <Link to="/lobby" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Go to the lobby
            </Link>
          </p>
        }
      >
        <p className="text-center text-sm text-spade-gray-2">
          You&apos;re all set — jump back in and take your seat.
        </p>
      </AuthCardShell>
    )
  }

  return (
    <AuthCardShell
      title="Verification failed"
      subtitle="We couldn't confirm this email link."
      footer={
        <p className="text-center text-sm text-spade-gray-3">
          <Link to="/lobby" className="font-medium text-spade-gold hover:text-spade-gold-light">
            Back to the lobby
          </Link>
        </p>
      }
    >
      <div role="alert" className={authErrorClassName}>
        {error}
      </div>
      <p className="mt-4 text-center text-sm text-spade-gray-2">
        Links expire after 24 hours. Open the latest email, or resend verification from your
        profile banner when signed in.
      </p>
    </AuthCardShell>
  )
}
