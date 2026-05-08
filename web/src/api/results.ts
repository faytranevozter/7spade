import { apiRequest } from './client'

export type ResultPlayerDto = {
  display_name: string
  penalty_points: number
  rank: number
  is_winner: boolean
  cards_left?: number
  facedown_count?: number
  voted_rematch?: boolean
}

export type RoomResultsResponse = {
  room_id: string
  results: ResultPlayerDto[]
  rematch_votes?: number
  rematch_total?: number
}

export function getRoomResults(token: string | null, roomId: string): Promise<RoomResultsResponse> {
  return apiRequest<RoomResultsResponse>(`/rooms/${encodeURIComponent(roomId)}/results`, { token })
}
