import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { postOAuthCallback, AuthApiError, type OAuthProvider } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

const errorMessages: Record<string, string> = {
  access_denied: 'You cancelled the sign-in.',
  invalid_or_expired_state: 'Sign-in session expired. Please try again.',
  state_provider_mismatch: 'Sign-in session could not be verified. Please try again.',
  token_exchange_failed: 'Could not complete sign-in with the provider. Please try again.',
  profile_fetch_failed: 'Could not load your profile from the provider. Please try again.',
  upsert_failed: 'Could not save your account. Please try again.',
  internal_error: 'Something went wrong. Please try again.',
}

function resolveErrorMessage(code: string): string {
  return errorMessages[code] ?? `Sign-in failed: ${code}`
}

export function OAuthCallbackPage() {
	const navigate = useNavigate()
	const location = useLocation()
	const { login } = useAuth()
	const handled = useRef(false)
	const [errorMessage, setErrorMessage] = useState<string | null>(null)
	const params = new URLSearchParams(location.search)
	const code = params.get('code')
	const state = params.get('state')
	const providerParam = params.get('provider') // set by the backend redirect URL config
	const errorParam = params.get('error')
	const initialErrorMessage = errorParam
		? resolveErrorMessage(errorParam)
		: !code || !state
			? 'Sign-in did not return a valid response. Please try again.'
			: null

	useEffect(() => {
		if (handled.current) return
		handled.current = true
		if (initialErrorMessage || !code || !state) {
			return
		}

    // Derive provider from the URL path segment, e.g. /auth/callback/google
    // or fall back to the query param if present.
    const pathParts = location.pathname.split('/')
    const providerFromPath = pathParts[pathParts.length - 1] as OAuthProvider
    const provider = (providerParam ?? providerFromPath) as OAuthProvider

    // Strip code/state from the URL before the async call completes
    if (typeof window !== 'undefined' && window.history.replaceState) {
      window.history.replaceState(null, '', window.location.pathname)
    }

    postOAuthCallback(provider, code, state)
      .then((res) => {
        if (!res.jwt) {
          setErrorMessage('Sign-in did not return a session token. Please try again.')
          return
        }
        login(res.jwt)
        navigate('/lobby', { replace: true })
      })
      .catch((err) => {
        if (err instanceof AuthApiError) {
          setErrorMessage(resolveErrorMessage(err.message))
        } else {
          setErrorMessage('An unexpected error occurred. Please try again.')
        }
      })
	}, [code, initialErrorMessage, location.pathname, login, navigate, providerParam, state])

	const visibleErrorMessage = initialErrorMessage ?? errorMessage

	return (
    <section className="grid min-h-svh place-items-center bg-spade-bg px-4">
      <div className="w-full max-w-md rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6 shadow-spade-card">
        <div className="flex items-center gap-3">
          <span className="grid size-9 place-items-center rounded-spade-md bg-spade-green-mid text-spade-gold-light">♠</span>
					<h2 className="text-xl font-medium">{visibleErrorMessage ? 'Sign-in failed' : 'Signing you in'}</h2>
				</div>
				{visibleErrorMessage ? (
					<>
						<p className="mt-3 text-sm text-spade-gray-2">{visibleErrorMessage}</p>
            <button
              type="button"
              onClick={() => navigate('/auth', { replace: true })}
              className="mt-4 inline-flex min-h-9 items-center justify-center rounded-spade-md bg-spade-gold px-4 py-2 text-sm font-medium text-[#1a0e00] transition hover:bg-[#d9a030] active:scale-95"
            >
              Back to sign in
            </button>
          </>
        ) : (
          <p className="mt-3 text-sm text-spade-gray-2">Verifying your account and taking you to the lobby…</p>
        )}
      </div>
    </section>
  )
}
