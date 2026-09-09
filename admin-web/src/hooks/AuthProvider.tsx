import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { login, logout, refresh, verifyMFA, type Admin } from '../api/auth'
import { setSessionRecovery } from '../api/client'
import { AuthContext } from './useAuth'

let refreshPromise: ReturnType<typeof refresh> | null = null

function getRefreshPromise() {
  if (!refreshPromise) {
    const pending = refresh().finally(() => {
      if (refreshPromise === pending) refreshPromise = null
    })
    refreshPromise = pending
  }
  return refreshPromise
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [admin, setAdmin] = useState<Admin | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [challengeToken, setChallengeToken] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState('')
  const accessToken = useRef<string | null>(null)
  const generation = useRef(0)

  useEffect(() => {
    const lifecycle = generation
    const expected = generation.current
    getRefreshPromise()
      .then((result) => {
        if (expected !== generation.current) return
        accessToken.current = result.access_token
        setAdmin(result.admin)
        setToken(result.access_token)
      })
      .catch(() => {})
      .finally(() => {
        if (expected === generation.current) setIsLoading(false)
      })
    return () => {
      lifecycle.current++
    }
  }, [])

  async function signIn(email: string, password: string) {
    const expected = ++generation.current
    refreshPromise = null
    setError('')
    try {
      const result = await login(email, password)
      if (expected !== generation.current) return
      if ('mfa_required' in result) {
        setChallengeToken(result.challenge_token)
        return
      }
      accessToken.current = result.access_token
      setAdmin(result.admin)
      setToken(result.access_token)
    } catch (requestError) {
      if (expected !== generation.current) return
      setError(
        requestError instanceof Error ? requestError.message : 'Sign in failed',
      )
    }
  }

  async function completeMFA(code: string) {
    const expected = ++generation.current
    setError('')
    try {
      const result = await verifyMFA(challengeToken, code)
      if (expected !== generation.current) return
      accessToken.current = result.access_token
      setChallengeToken('')
      setAdmin(result.admin)
      setToken(result.access_token)
    } catch (requestError) {
      if (expected !== generation.current) return
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Verification failed',
      )
    }
  }

  const refreshSession = useCallback(async () => {
    const expected = generation.current
    if (!accessToken.current) throw new Error('Administrator session expired')
    const result = await getRefreshPromise()
    if (expected !== generation.current)
      throw new Error('Administrator session expired')
    // Keep the context token stable: page effects use it as a session identity.
    accessToken.current = result.access_token
    return result.access_token
  }, [])

  const expireSession = useCallback((message: string) => {
    generation.current++
    refreshPromise = null
    accessToken.current = null
    setSessionRecovery(null)
    setAdmin(null)
    setToken(null)
    setChallengeToken('')
    setIsLoading(false)
    setError(message)
  }, [])

  useEffect(() => {
    setSessionRecovery(refreshSession, () => accessToken.current, expireSession)
    return () => setSessionRecovery(null)
  }, [expireSession, refreshSession, token])

  async function signOut() {
    expireSession('')
    const expected = generation.current
    try {
      await logout()
    } catch (requestError) {
      if (expected !== generation.current) return
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Sign out failed',
      )
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
