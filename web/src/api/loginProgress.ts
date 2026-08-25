import type { SkinGrantDto } from './auth'
import { apiRequest } from './client'

export type LoginStreakResponse = {
  current_streak: number
  best_streak: number
  last_claim_date: string | null
  claimed_today: boolean
  new_skin_grants: SkinGrantDto[]
}

export function getLoginStreak(token: string | null): Promise<LoginStreakResponse> {
  return apiRequest<LoginStreakResponse>('/me/login-streak', { token })
}

export function claimLoginStreak(token: string | null): Promise<LoginStreakResponse> {
  return apiRequest<LoginStreakResponse>('/me/login-streak/claim', { method: 'POST', token })
}
