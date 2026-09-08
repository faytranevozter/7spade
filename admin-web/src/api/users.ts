import { apiResponse } from './client'

export type User = {
  id: string
  username: string
  display_name: string
  version: number
  email?: string
  created_at: string
  online: boolean
  suspension?: { reason: string; expires_at?: string }
}

export type UserPage = { users: User[]; limit: number; offset: number }
export type UserAchievement = {
  achievement_id: string
  name: string
  description: string
  icon: string
  earned_at: string
}
export type UserSkin = {
  id: string
  name: string
  skin_type: string
  source: string
  revision_id?: string
  asset_url: string
  earned_at: string
}
export type UserDetail = {
  user: User
  providers: string[]
  stats: Partial<
    Record<'games_played' | 'wins' | 'total_penalty' | 'xp', number>
  >
  ratings: Array<{
    rating_before: number
    rating_after: number
    rating_delta: number
    created_at: string
  }>
  achievements: UserAchievement[]
  skins: UserSkin[]
  games: Array<{
    id: string
    room_id: string
    finished_at: string | null
    penalty_points: number
    rank: number
  }>
  room?: { id: string; status: string; created_at: string }
}

export function searchUsers(
  token: string,
  query: string,
  limit = 50,
  offset = 0,
) {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })
  if (query) params.set('query', query)
  return apiResponse<UserPage>(`/users?${params}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
}

export function getUser(token: string, id: string) {
  return apiResponse<UserDetail>(`/users/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((detail) => ({
    ...detail,
    stats: detail.stats ?? {},
    providers: detail.providers ?? [],
    ratings: detail.ratings ?? [],
    achievements: detail.achievements ?? [],
    skins: detail.skins ?? [],
    games: detail.games ?? [],
  }))
}

export type ModerationResponse = {
  suspension?: NonNullable<User['suspension']>
  audit_action: string
  audit_event_id: string
}

export function suspendUser(
  token: string,
  id: string,
  reason: string,
  expiresAt?: string,
) {
  return apiResponse<ModerationResponse>(`/users/${id}/suspension`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      reason,
      ...(expiresAt ? { expires_at: expiresAt } : {}),
    }),
  })
}

export function reinstateUser(token: string, id: string) {
  return apiResponse<ModerationResponse>(`/users/${id}/suspension`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  })
}

export function updateUserDisplayName(
  token: string,
  id: string,
  displayName: string,
  reason: string,
  version: number,
) {
  return apiResponse<ModerationResponse & { user: User }>(
    `/users/${id}/display-name`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ display_name: displayName, reason, version }),
    },
  )
}
