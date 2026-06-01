import { createContext, useContext } from 'react'

// Auth context shape, ported from web/src/hooks/useAuth.ts and extended for
// native: login carries an optional refresh token (persisted to SecureStore),
// and `isLoading` covers the async session hydration on cold start (the web app
// reads sessionStorage synchronously, native storage is async).
export interface UseAuthReturn {
  token: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (token: string, refreshToken?: string | null) => void
  logout: () => void
}

export const AuthContext = createContext<UseAuthReturn | null>(null)

export function useAuth(): UseAuthReturn {
  const context = useContext(AuthContext)
  if (context === null) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
