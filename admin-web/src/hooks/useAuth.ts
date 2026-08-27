import { createContext, useContext } from 'react'
import type { Admin } from '../api/auth'

export type AuthState = {
  admin: Admin | null
  token: string | null
  challengeToken: string
  isLoading: boolean
  error: string
  signIn: (email: string, password: string) => Promise<void>
  completeMFA: (code: string) => Promise<void>
  signOut: () => Promise<void>
  refreshSession: () => Promise<string>
  expireSession: (message: string) => void
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within an AuthProvider')
  return context
}
