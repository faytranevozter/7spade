import { apiRequest } from './client'

export type HistoryGameDto = {
  game_id: string
  room_id: string
  room_name: string
  started_at: string
  finished_at: string
  penalty_points: number
  rank: number
  is_winner: boolean
  rating_delta?: number | null
  replay_available?: boolean
  results_available?: boolean
}

export type GameResultCardDto = {
  suit: string
  rank: number
  points: number
}

export type GameResultPlayerDto = {
  player_index: number
  user_id?: string | null
  display_name: string
  penalty_points: number
  rank: number
  is_winner: boolean
  is_bot: boolean
  is_guest: boolean
  is_me: boolean
  team?: number
  facedown_cards: GameResultCardDto[]
  rating_delta?: number
  rating_after?: number
  xp_delta?: number
  xp_after?: number
  level?: number
}

export type GameResultsDto = {
  game_id: string
  room_id: string
  room_name: string
  started_at: string
  finished_at: string
  replay_available: boolean
  players: GameResultPlayerDto[]
}

export type HistoryResponse = {
  games: HistoryGameDto[]
  total: number
  page: number
}

export function getHistory(token: string | null, page: number, perPage: number): Promise<HistoryResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })

  return apiRequest<HistoryResponse>(`/history?${params.toString()}`, { token })
}

export function getGameResults(token: string | null, gameId: string): Promise<GameResultsDto> {
  return apiRequest<GameResultsDto>(`/games/${encodeURIComponent(gameId)}/results`, { token })
}
