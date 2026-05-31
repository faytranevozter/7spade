import { apiRequest } from './client'

export type EarnedAchievementDto = {
  achievement_id: string
  earned_at: string
}

export type AchievementsResponse = {
  earned: EarnedAchievementDto[]
  catalog: string[]
}

export function getUserAchievements(
  token: string | null,
  userId: string,
): Promise<AchievementsResponse> {
  return apiRequest<AchievementsResponse>(`/users/${encodeURIComponent(userId)}/achievements`, { token })
}
