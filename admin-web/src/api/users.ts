import { apiResponse } from './client'

export type User = {
  id: string
  username: string
  display_name: string
  email?: string
  created_at: string
  online: boolean
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
  return apiResponse<UserDetail>(`/users/${id}`, { headers: { Authorization: `Bearer ${token}` } })
}
