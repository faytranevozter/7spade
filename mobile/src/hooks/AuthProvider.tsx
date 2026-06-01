import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { AuthContext, type UseAuthReturn } from './useAuth'
import { clearSession, loadSession, saveSession } from '../auth/storage'
import { decodeJwtClaims } from '../auth/claims'
import { postRefresh } from '../api/auth'

// AuthProvider holds the access token + refresh token in shared React state,
// mirrored to SecureStore. On cold start it hydrates from storage and, if the
// stored access JWT is expired but a refresh token exists, transparently rotates
// it via /refresh before marking the session ready. (The web app relies on a
// HttpOnly cookie + manual refresh; native must drive this itself.)
export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null)
  const [refreshToken, setRefreshToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  // Guards against a late hydration overwriting an explicit login/logout that
  // happened while storage was still loading.
  const mutatedRef = useRef(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const stored = await loadSession()
        if (cancelled || mutatedRef.current) return

        let nextToken = stored.token
        let nextRefresh = stored.refreshToken

        // If the access token is missing/expired but we hold a refresh token,
        // rotate it so the user stays signed in across launches.
        if ((!nextToken || isExpired(nextToken)) && nextRefresh) {
          try {
            const refreshed = await postRefresh(nextRefresh)
            if (cancelled || mutatedRef.current) return
            nextToken = refreshed.jwt
            nextRefresh = refreshed.refreshToken ?? nextRefresh
            await saveSession({ token: nextToken, refreshToken: nextRefresh })
          } catch {
            // Refresh failed (revoked/expired) — drop the stale session.
            nextToken = null
            nextRefresh = null
            await clearSession()
          }
        }

        if (cancelled || mutatedRef.current) return
        setToken(nextToken)
        setRefreshToken(nextRefresh)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback((newToken: string, newRefresh?: string | null) => {
    mutatedRef.current = true
    setToken(newToken)
    setRefreshToken(newRefresh ?? null)
    void saveSession({ token: newToken, refreshToken: newRefresh ?? null })
  }, [])

  const logout = useCallback(() => {
    mutatedRef.current = true
    setToken(null)
    setRefreshToken(null)
    void clearSession()
  }, [])

  const value = useMemo<UseAuthReturn>(
    () => ({
      token,
      refreshToken,
      isAuthenticated: token !== null && token.length > 0,
      isLoading,
      login,
      logout,
    }),
    [token, refreshToken, isLoading, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// isExpired checks the JWT `exp` claim (seconds since epoch) with a small skew.
function isExpired(token: string): boolean {
  const parts = token.split('.')
  if (parts.length < 2) return true
  try {
    // Reuse the claims decoder's base64url handling indirectly: decodeJwtClaims
    // only exposes identity fields, so parse exp here with the same approach.
    const payload = JSON.parse(
      base64UrlDecode(parts[1]),
    ) as { exp?: number }
    if (typeof payload.exp !== 'number') return false
    return payload.exp * 1000 <= Date.now() + 5000
  } catch {
    return true
  }
}

// Minimal base64url -> string decode (Hermes lacks a reliable atob). Mirrors the
// decoder in auth/claims.ts; kept local to avoid widening that module's surface.
const BASE64_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
function base64UrlDecode(input: string): string {
  const clean = input.replace(/-/g, '+').replace(/_/g, '/').replace(/[^A-Za-z0-9+/]/g, '')
  let buffer = 0
  let bits = 0
  let result = ''
  for (const char of clean) {
    const value = BASE64_CHARS.indexOf(char)
    if (value === -1) continue
    buffer = (buffer << 6) | value
    bits += 6
    if (bits >= 8) {
      bits -= 8
      result += String.fromCharCode((buffer >> bits) & 0xff)
    }
  }
  return result
}

// Re-export so screens can read identity without importing the claims module
// directly when they already depend on the provider.
export { decodeJwtClaims }
