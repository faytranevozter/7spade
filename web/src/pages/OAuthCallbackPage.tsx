import { useEffect, useMemo, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { parseOAuthCallbackFragment } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

const errorMessages: Record<string, string> = {
  access_denied: 'You cancelled the sign-in.',
  missing_state_or_code: 'OAuth response was incomplete. Please try again.',
  missing_state_cookie: 'Sign-in session expired. Please try again.',
  state_mismatch: 'Sign-in session could not be verified. Please try again.',
  state_invalid: 'Sign-in session was tampered with. Please try again.',
  token_exchange_failed: 'Could not complete sign-in with the provider. Please try again.',
  profile_fetch_failed: 'Could not load your profile from the provider. Please try again.',
  no_email: 'The provider did not return an email address. Try a different sign-in method.',
  upsert_failed: 'Could not save your account. Please try again.',
  jwt_failed: 'Could not issue a session. Please try again.',
  refresh_failed: 'Could not issue a session. Please try again.',
  store_refresh_failed: 'Could not save your session. Please try again.',
}

function resolveErrorMessage(code: string): string {
  return errorMessages[code] ?? `Sign-in failed: ${code}`
}

export function OAuthCallbackPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const handled = useRef(false)

  // Compute the OAuth result synchronously so we don't have to setState inside an effect.
  // Falling back to window.location.hash keeps real-browser navigation working when the
  // router hasn't been seeded with the hash (e.g. after an external redirect).
  const result = useMemo(() => {
    const hash = location.hash || (typeof window !== 'undefined' ? window.location.hash : '')
    return parseOAuthCallbackFragment(hash)
  }, [location.hash])

  const errorMessage = result.error
    ? resolveErrorMessage(result.error)
    : !result.jwt
      ? 'Sign-in did not return a session token. Please try again.'
      : null

  useEffect(() => {
    if (handled.current) {
      return
    }
    handled.current = true

    // Strip the fragment so the JWT doesn't sit in the URL on subsequent reloads.
    if (typeof window !== 'undefined' && window.history.replaceState) {
      window.history.replaceState(null, '', window.location.pathname + window.location.search)
    }

    if (errorMessage) {
      return
    }

    if (result.jwt) {
      login(result.jwt, result.refreshToken)
      navigate('/lobby', { replace: true })
    }
  }, [errorMessage, login, navigate, result.jwt, result.refreshToken])

  return (
    <section className="grid min-h-svh place-items-center bg-spade-bg px-4">
      <div className="w-full max-w-md rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6 shadow-spade-card">
        <div className="flex items-center gap-3">
          <span className="grid size-9 place-items-center rounded-spade-md bg-spade-green-mid text-spade-gold-light">♠</span>
          <h2 className="text-xl font-medium">{errorMessage ? 'Sign-in failed' : 'Signing you in'}</h2>
        </div>
        {errorMessage ? (
          <>
            <p className="mt-3 text-sm text-spade-gray-2">{errorMessage}</p>
            <button
              type="button"
              onClick={() => navigate('/auth', { replace: true })}
              className="mt-4 inline-flex min-h-9 items-center justify-center rounded-spade-md bg-spade-gold px-4 py-2 text-sm font-medium text-[#1a0e00] transition hover:bg-[#d9a030] active:scale-95"
            >
              Back to sign in
            </button>
          </>
        ) : (
          <p className="mt-3 text-sm text-spade-gray-2">Verifying your account and taking you to the lobby...</p>
        )}
      </div>
    </section>
  )
}
