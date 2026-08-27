import { apiResponse, csrfHeaders } from './client'

export type Role = {
  id: string
  name: string
  description: string
  permissions: string[]
}

export type Permission = {
  name: string
  description: string
}

export type Admin = {
  id: string
  email: string
  display_name: string
  status: string
  roles?: Role[]
  permissions: string[]
  mfa_enrolled?: boolean
  created_at?: string
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

export type Invitation = {
  id: string
  email: string
  role_id: string
  role_name?: string
  invited_by?: string
  expires_at: string
  created_at: string
}

export type InviteAdminResponse = {
  invitation: Invitation
  token: string
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

export function getAdmins(token: string) {
  return apiResponse<Admin[]>('/admins', { headers: { Authorization: `Bearer ${token}` } })
}

export function inviteAdmin(token: string, email: string, roleId: string) {
  return apiResponse<InviteAdminResponse>('/admins/invite', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ email, role_id: roleId }),
  })
}

export function setAdminStatus(token: string, adminId: string, status: 'active' | 'disabled') {
  return apiResponse<void>(`/admins/${adminId}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ status }),
  })
}

export function setAdminRoles(token: string, adminId: string, roleIds: string[]) {
  return apiResponse<void>(`/admins/${adminId}/roles`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ role_ids: roleIds }),
  })
}

export function getRoles(token: string) {
  return apiResponse<Role[]>('/roles', { headers: { Authorization: `Bearer ${token}` } })
}

export function getPermissions(token: string) {
  return apiResponse<Permission[]>('/permissions', { headers: { Authorization: `Bearer ${token}` } })
}

export function updateRolePermissions(token: string, roleId: string, permissions: string[]) {
  return apiResponse<void>(`/roles/${roleId}/permissions`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ permissions }),
  })
}
