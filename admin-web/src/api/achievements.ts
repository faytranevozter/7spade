import { apiResponse } from './client'

export type AchievementRule = { metric: string; operator: 'eq' | 'gte' | 'lte' | 'gt' | 'lt'; value: string }
export type Achievement = {
  id: string
  name: string
  description: string
  icon: string
  display_order: number
  enabled: boolean
  rules: AchievementRule[]
  rules_locked: boolean
}

const headers = (token: string) => ({ 'Content-Type': 'application/json', Authorization: `Bearer ${token}` })

export const getAchievements = (token: string) => apiResponse<{ achievements: Achievement[] }>('/achievements', { headers: { Authorization: `Bearer ${token}` } })
export const createAchievement = (token: string, achievement: Achievement, reason: string) =>
  apiResponse<Achievement>('/achievements', {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({ ...achievement, reason }),
  })
export const saveAchievement = (token: string, achievement: Achievement, reason: string) =>
  apiResponse<Achievement>(`/achievements/${achievement.id}`, {
    method: 'PUT',
    headers: headers(token),
    body: JSON.stringify({ ...achievement, reason }),
  })
export const changeAchievementEntitlement = (
  token: string,
  userID: string,
  achievementID: string,
  action: 'grant' | 'revoke',
  reason: string,
  idempotencyKey: string,
) => apiResponse(`/users/${userID}/achievements/${achievementID}/${action}`, {
  method: 'POST', headers: headers(token), body: JSON.stringify({ reason, idempotency_key: idempotencyKey }),
})
