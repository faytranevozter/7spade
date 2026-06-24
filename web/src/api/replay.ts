import { apiRequest } from './client'

export type ReplayCardDto = {
  suit: string
  rank: number
}

export type ReplayMoveDto = {
  index: number
  player_index: number
  suit: string
  rank: number
  type: 'play' | 'face_down' | 'ace_close'
  ace_direction?: 'low' | 'high' | ''
}

export type ReplayPlayerDto = {
  player_index: number
  display_name: string
  is_bot: boolean
  is_winner: boolean
  rank: number
}

export type ReplayDto = {
  game_id: string
  room_name: string
  started_at: string
  finished_at: string
  players: ReplayPlayerDto[]
  initial_hands: ReplayCardDto[][]
  moves: ReplayMoveDto[]
}

export function getReplay(token: string | null, gameId: string): Promise<ReplayDto> {
  return apiRequest<ReplayDto>(`/games/${encodeURIComponent(gameId)}/replay`, { token })
}
