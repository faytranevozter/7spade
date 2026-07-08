import { apiRequest } from './client'

export type EarnedAchievementDto = {
  achievement_id: string
  earned_at: string
}

export type AchievementDto = {
  id: string
  name: string
  description: string
  icon: string
}

export type AchievementsResponse = {
  earned: EarnedAchievementDto[]
  // Server-authoritative achievement catalog. The frontend intentionally has no
  // bundled fallback so catalog/order/enablement cannot drift from the API DB.
  catalog: AchievementDto[]
}

export function getUserAchievements(
  token: string | null,
  userId: string,
): Promise<AchievementsResponse> {
  return apiRequest<AchievementsResponse>(`/users/${encodeURIComponent(userId)}/achievements`, { token })
}
