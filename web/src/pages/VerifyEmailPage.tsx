import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { SceneShell } from '../components/SceneShell'
import { postVerifyEmail, AuthApiError } from '../api/auth'

type Status = 'verifying' | 'success' | 'error'

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''
  const [status, setStatus] = useState<Status>(token ? 'verifying' : 'error')
  const [error, setError] = useState<string | null>(token ? null : 'This verification link is missing its token.')
  // Guard against React StrictMode double-invoke consuming the single-use token twice.
  const startedRef = useRef(false)

  useEffect(() => {
    if (!token || startedRef.current) return
    startedRef.current = true
    postVerifyEmail(token)
      .then(() => setStatus('success'))
      .catch((err) => {
        setStatus('error')
        setError(err instanceof AuthApiError ? err.message : 'Verification failed. The link may have expired.')
      })
  }, [token])

  return (
    <SceneShell title="Verify email" eyebrow="Account">
      <div className="mx-auto max-w-md">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-6 text-center">
          {status === 'verifying' && (
            <>
              <h2 className="text-2xl font-medium">Verifying…</h2>
              <p className="mt-2 text-sm text-spade-gray-2">Confirming your email address.</p>
            </>
          )}
          {status === 'success' && (
            <>
              <h2 className="text-2xl font-medium">Email verified</h2>
              <p className="mt-2 text-sm text-spade-gray-2">Thanks! Your email address is now verified.</p>
              <Link to="/lobby" className="mt-6 inline-block text-spade-gold hover:text-spade-gold-light">Go to the lobby</Link>
            </>
          )}
          {status === 'error' && (
            <>
              <h2 className="text-2xl font-medium">Verification failed</h2>
              <p role="alert" className="mt-2 text-sm text-spade-red">{error}</p>
              <Link to="/lobby" className="mt-6 inline-block text-spade-gold hover:text-spade-gold-light">Back to the lobby</Link>
            </>
          )}
        </div>
      </div>
    </SceneShell>
  )
}
