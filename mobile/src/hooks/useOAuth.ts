import { useCallback, useState } from 'react'
import * as WebBrowser from 'expo-web-browser'
import * as Linking from 'expo-linking'
import {
  getOAuthStartUrl,
  postOAuthCallback,
  AuthApiError,
  type AuthResponse,
  type OAuthProvider,
} from '../api/auth'

// useOAuth drives the native deep-link OAuth flow:
//  1. build the app redirect URI (sevenspade://auth/callback),
//  2. ask the API for the provider authorize URL (the API holds the PKCE
//     verifier in Redis),
//  3. open it in a system auth session and wait for the deep-link redirect,
//  4. extract code + state from the redirect and exchange them for an app JWT.
//
// The session token never leaves the API's control on the client side — we only
// pass code/state through.

// Ensures any in-flight auth session is completed (no-op on native cold start,
// required for web popups).
WebBrowser.maybeCompleteAuthSession()

export type UseOAuthReturn = {
  signIn: (provider: OAuthProvider) => Promise<AuthResponse | null>
  isLoading: boolean
  error: string | null
}

export function useOAuth(): UseOAuthReturn {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const signIn = useCallback(async (provider: OAuthProvider): Promise<AuthResponse | null> => {
    setError(null)
    setIsLoading(true)
    try {
      // Deep-link URI the provider redirects back to. Registered with each
      // provider and allowed by the API (see services/api oauth handler).
      const redirectUri = Linking.createURL('auth/callback', { queryParams: { provider } })

      const { url } = await getOAuthStartUrl(provider, redirectUri)

      const result = await WebBrowser.openAuthSessionAsync(url, redirectUri)
      if (result.type !== 'success' || !result.url) {
        // User dismissed the browser or it was cancelled.
        return null
      }

      const parsed = Linking.parse(result.url)
      const code = paramString(parsed.queryParams?.code)
      const state = paramString(parsed.queryParams?.state)
      const providerError = paramString(parsed.queryParams?.error)

      if (providerError) {
        setError(resolveErrorMessage(providerError))
        return null
      }
      if (!code || !state) {
        setError('Sign-in did not return a valid response. Please try again.')
        return null
      }

      const response = await postOAuthCallback(provider, code, state, redirectUri)
      if (!response.jwt) {
        setError('Sign-in did not return a session token. Please try again.')
        return null
      }
      return response
    } catch (err) {
      if (err instanceof AuthApiError) {
        setError(resolveErrorMessage(err.message))
      } else if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('An unexpected error occurred. Please try again.')
      }
      return null
    } finally {
      setIsLoading(false)
    }
  }, [])

  return { signIn, isLoading, error }
}

function paramString(value: string | string[] | undefined): string | null {
  if (Array.isArray(value)) return value[0] ?? null
  return value ?? null
}

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
