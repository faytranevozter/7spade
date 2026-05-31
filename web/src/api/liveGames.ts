import { apiRequest } from './client'

export type LiveGamePlayerDto = {
  user_id: string
  display_name: string
}

export type LiveGameDto = {
  room_id: string
  invite_code: string
  started_at: string
  player_count: number
  players: LiveGamePlayerDto[]
}

export type LiveGamesResponse = {
  games: LiveGameDto[]
}

export function getLiveGames(token: string | null): Promise<LiveGamesResponse> {
  return apiRequest<LiveGamesResponse>('/live-games', { token })
}
