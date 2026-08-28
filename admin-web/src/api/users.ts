import { apiResponse } from './client'

export type User = {
  id: string
  username: string
  display_name: string
  email?: string
  created_at: string
  online: boolean
  suspension?: { reason: string; expires_at?: string }
}

export type UserPage = { users: User[]; limit: number; offset: number }
export type UserDetail = {
  user: User
  providers: string[]
  stats: Record<string, unknown>
  ratings: Array<Record<string, unknown>>
  achievements: Array<Record<string, unknown>>
  skins: Array<Record<string, unknown>>
  games: Array<Record<string, unknown>>
  room?: Record<string, unknown>
}

export function searchUsers(token: string, query: string, limit = 50, offset = 0) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  if (query) params.set('query', query)
  return apiResponse<UserPage>(`/users?${params}`, { headers: { Authorization: `Bearer ${token}` } })
}

export function getUser(token: string, id: string) {
  return apiResponse<UserDetail>(`/users/${id}`, { headers: { Authorization: `Bearer ${token}` } }).then((detail) => ({
    ...detail,
    providers: detail.providers ?? [],
    ratings: detail.ratings ?? [],
    achievements: detail.achievements ?? [],
    skins: detail.skins ?? [],
    games: detail.games ?? [],
  }))
}

export type ModerationResponse = { suspension?: NonNullable<User['suspension']>; audit_action: string; audit_event_id: string }

export function suspendUser(token: string, id: string, reason: string, expiresAt?: string) {
  return apiResponse<ModerationResponse>(`/users/${id}/suspension`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ reason, ...(expiresAt ? { expires_at: expiresAt } : {}) }),
  })
}

export function reinstateUser(token: string, id: string) {
  return apiResponse<ModerationResponse>(`/users/${id}/suspension`, {
    method: 'DELETE', headers: { Authorization: `Bearer ${token}` },
  })
}
