import { apiRequest } from './client'

export type HistoryGameDto = {
  game_id: string
  room_id: string
  started_at: string
  finished_at: string
  penalty_points: number
  rank: number
  is_winner: boolean
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
