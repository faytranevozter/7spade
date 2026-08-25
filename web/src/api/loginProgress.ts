import type { DailyLoginRewardDto } from './auth'
import { apiRequest } from './client'

export type LoginStreakResponse = DailyLoginRewardDto

export function getLoginStreak(token: string | null): Promise<LoginStreakResponse> {
  return apiRequest<LoginStreakResponse>('/me/login-streak', { token })
}

export function claimLoginStreak(token: string | null): Promise<LoginStreakResponse> {
  return apiRequest<LoginStreakResponse>('/me/login-streak/claim', { method: 'POST', token })
}
