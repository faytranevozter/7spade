import { apiResponse, csrfHeaders } from './client'

export type Admin = {
  id: string
  email: string
  display_name: string
  status: string
  permissions: string[]
  mfa_enrolled?: boolean
}

export type AuthResponse = { access_token: string; admin: Admin }
export type MFAChallenge = { mfa_required: true; challenge_token: string }

export function login(email: string, password: string) {
  return apiResponse<AuthResponse | MFAChallenge>('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
}

export function verifyMFA(challengeToken: string, code: string) {
  const normalizedCode = code.replace(/[\s-]/g, '')
  const credential = normalizedCode.length === 6 ? { code: normalizedCode } : { recovery_code: code }
  return apiResponse<AuthResponse>('/auth/mfa/challenge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ challenge_token: challengeToken, ...credential }),
  })
}

export function refresh() {
  return apiResponse<AuthResponse>('/auth/refresh', { method: 'POST', headers: csrfHeaders() })
}

export function logout() {
  return apiResponse<void>('/auth/logout', { method: 'DELETE', headers: csrfHeaders() })
}
