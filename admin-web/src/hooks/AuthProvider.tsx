import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { login, logout, refresh, verifyMFA, type Admin } from '../api/auth'
import { AuthContext } from './useAuth'

let refreshPromise: ReturnType<typeof refresh> | null = null

function getRefreshPromise() {
  if (!refreshPromise) {
    refreshPromise = refresh().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [admin, setAdmin] = useState<Admin | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [challengeToken, setChallengeToken] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getRefreshPromise()
      .then((result) => {
        setAdmin(result.admin)
        setToken(result.access_token)
      })
      .catch(() => {})
      .finally(() => setIsLoading(false))
  }, [])

  async function signIn(email: string, password: string) {
    setError('')
    try {
      const result = await login(email, password)
      if ('mfa_required' in result) {
        setChallengeToken(result.challenge_token)
        return
      }
      setAdmin(result.admin)
      setToken(result.access_token)
    } catch (requestError) {
      setError(
        requestError instanceof Error ? requestError.message : 'Sign in failed',
      )
    }
  }

  async function completeMFA(code: string) {
    setError('')
    try {
      const result = await verifyMFA(challengeToken, code)
      setChallengeToken('')
      setAdmin(result.admin)
      setToken(result.access_token)
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Verification failed',
      )
    }
  }

  const refreshSession = useCallback(async () => {
    const result = await getRefreshPromise()
    setAdmin(result.admin)
    setToken(result.access_token)
    return result.access_token
  }, [])

  const expireSession = useCallback((message: string) => {
    setAdmin(null)
    setToken(null)
    setError(message)
  }, [])

  async function signOut() {
    setError('')
    try {
      await logout()
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Sign out failed',
      )
    } finally {
      setAdmin(null)
      setToken(null)
    }
  }

  return (
    <AuthContext.Provider
      value={{
        admin,
        token,
        challengeToken,
        isLoading,
        error,
        signIn,
        completeMFA,
        signOut,
        refreshSession,
        expireSession,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
