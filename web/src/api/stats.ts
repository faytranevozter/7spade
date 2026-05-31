import { apiRequest } from './client'

export type LeaderboardEntryDto = {
  rank: number
  user_id: string
  display_name: string
  games_played: number
  wins: number
  win_rate: number
  avg_penalty: number
  best_penalty: number | null
}

export type LeaderboardResponse = {
  entries: LeaderboardEntryDto[]
  total: number
  page: number
  min_games: number
}

export type UserStatsDto = {
  user_id: string
  display_name: string
  games_played: number
  wins: number
  win_rate: number
  avg_penalty: number
  best_penalty: number | null
  rank: number | null
  qualified: boolean
}

export function getLeaderboard(
  token: string | null,
  page: number,
  perPage: number,
): Promise<LeaderboardResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })

  return apiRequest<LeaderboardResponse>(`/leaderboard?${params.toString()}`, { token })
}

export function getMyStats(token: string | null): Promise<UserStatsDto> {
  return apiRequest<UserStatsDto>('/stats', { token })
}

export function getUserStats(token: string | null, userId: string): Promise<UserStatsDto> {
  return apiRequest<UserStatsDto>(`/users/${encodeURIComponent(userId)}/stats`, { token })
}
