import { apiRequest } from './client'

export type LeaderboardSort =
  | 'win_rate'
  | 'total_wins'
  | 'avg_penalty'
  | 'best_penalty'
  | 'games_played'
  | 'rating'
  | 'avg_rank'
  | 'top2_rate'

export const LEADERBOARD_SORTS: { value: LeaderboardSort; label: string }[] = [
  { value: 'win_rate', label: 'Win Rate' },
  { value: 'total_wins', label: 'Total Wins' },
  { value: 'avg_penalty', label: 'Avg Penalty' },
  { value: 'best_penalty', label: 'Best Penalty' },
  { value: 'games_played', label: 'Games Played' },
  { value: 'rating', label: 'Rating' },
  { value: 'avg_rank', label: 'Avg Rank' },
  { value: 'top2_rate', label: 'Top 2 Rate' },
]

export const DEFAULT_LEADERBOARD_SORT: LeaderboardSort = 'win_rate'

export function isLeaderboardSort(value: string | null): value is LeaderboardSort {
  return LEADERBOARD_SORTS.some((sort) => sort.value === value)
}

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
  avg_rank: number
  top2_rate: number
  first_place_count: number
  human_only_games: number
  bot_mixed_games: number
}

export type LeaderboardResponse = {
  entries: LeaderboardEntryDto[]
  total: number
  page: number
  min_games: number
  sort: LeaderboardSort
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
  worst_penalty: number | null
  rating: number
  rank: number | null
  qualified: boolean
  avg_rank: number
  first_place_count: number
  second_place_count: number
  third_place_count: number
  fourth_place_count: number
  zero_penalty_games: number
  low_penalty_games: number
  high_penalty_games: number
  human_only_games: number
  bot_mixed_games: number
  current_win_streak: number
  best_win_streak: number
  current_top2_streak: number
  best_top2_streak: number
  close_wins: number
  close_losses: number
  blowout_wins: number
  blowout_losses: number
}

export type RatingEventDto = {
  game_id: string
  rating_before: number
  rating_after: number
  rating_delta: number
  created_at: string
}

export type RatingHistoryResponse = {
  events: RatingEventDto[]
  total: number
  page: number
}

export function getLeaderboard(
  token: string | null,
  page: number,
  perPage: number,
  sort: LeaderboardSort = DEFAULT_LEADERBOARD_SORT,
  season: SeasonScope = '',
): Promise<LeaderboardResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
    sort,
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

export function getRatingHistory(
  token: string | null,
  userId: string,
  page: number = 1,
  perPage: number = 20,
): Promise<RatingHistoryResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })
  return apiRequest<RatingHistoryResponse>(`/users/${encodeURIComponent(userId)}/rating-history?${params.toString()}`, { token })
}
