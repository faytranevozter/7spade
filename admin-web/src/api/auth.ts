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
export type AdminSession = {
  id: string
  created_at: string
  expires_at: string
  ip_address: string
  user_agent: string
  current: boolean
}

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

export function getSessions(token: string) {
  return apiResponse<AdminSession[]>('/sessions', { headers: { Authorization: `Bearer ${token}` } })
}

export function revokeSession(token: string, id: string) {
  return apiResponse<void>(`/sessions/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
}

export function revokeOtherSessions(token: string) {
  return apiResponse<void>('/sessions/others', { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
}
