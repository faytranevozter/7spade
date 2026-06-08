import { apiRequest } from './client'

// Season scope for leaderboard / stats queries. '' (or 'all') is the all-time
// view (default); 'active' resolves to the current open season server-side; a
// concrete id like '2026-06' selects that month.
export type SeasonScope = string

export type SeasonDto = {
  id: string
  label: string
  started_at: string
  ended_at: string | null
  active: boolean
}

export type SeasonsResponse = {
  seasons: SeasonDto[]
}

export type LeaderboardEntryDto = {
  rank: number
  user_id: string
  display_name: string
  avatar_url: string | null
  games_played: number
  wins: number
  win_rate: number
  avg_penalty: number
  best_penalty: number | null
  rating: number
}

export type LeaderboardResponse = {
  entries: LeaderboardEntryDto[]
  total: number
  page: number
  min_games: number
  season: string
}

export type UserStatsDto = {
  user_id: string
  display_name: string
  avatar_url: string | null
  games_played: number
  wins: number
  win_rate: number
  avg_penalty: number
  best_penalty: number | null
  rating: number
  rank: number | null
  qualified: boolean
}

export function getLeaderboard(
  token: string | null,
  page: number,
  perPage: number,
  season: SeasonScope = '',
): Promise<LeaderboardResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })
  if (season) {
    params.set('season', season)
  }

  return apiRequest<LeaderboardResponse>(`/leaderboard?${params.toString()}`, { token })
}

export function getSeasons(token: string | null): Promise<SeasonsResponse> {
  return apiRequest<SeasonsResponse>('/seasons', { token })
}

export function getMyStats(token: string | null, season: SeasonScope = ''): Promise<UserStatsDto> {
  const query = season ? `?season=${encodeURIComponent(season)}` : ''
  return apiRequest<UserStatsDto>(`/stats${query}`, { token })
}

export function getUserStats(
  token: string | null,
  userId: string,
  season: SeasonScope = '',
): Promise<UserStatsDto> {
  const query = season ? `?season=${encodeURIComponent(season)}` : ''
  return apiRequest<UserStatsDto>(`/users/${encodeURIComponent(userId)}/stats${query}`, { token })
}
