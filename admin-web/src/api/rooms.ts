import { apiResponse } from './client'

export type Room = {
  id: string
  invite_code: string
  name: string
  status: string
  visibility: string
  game_mode: string
  practice_mode: boolean
  max_players: number
  deck_count: number
  scoring_mode: string
  team_mode: string
  turn_timer_seconds: number
  created_by: string
  created_at: string
  player_count: number
}

export type RoomPlayer = { user_id: string; display_name: string; joined_at: string }
export type LiveRoomSummary = { role: string; phase: string; players: Array<{ user_id: string; display_name: string; connected: boolean; is_bot?: boolean; seat?: number }>; turn_deadline?: string; snapshot_age_seconds: number; state_version: number; owner_id: string; fence_token: number }
export type RoomDetail = { room: Room; players: RoomPlayer[]; live: { available: boolean; reason?: string; summary?: LiveRoomSummary } }
export type RoomFilters = { id?: string; invite_code?: string; status?: string; visibility?: string; mode?: string; created_from?: string; created_to?: string }

export function searchRooms(token: string, filters: RoomFilters, limit = 50, offset = 0) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  for (const [key, value] of Object.entries(filters)) if (value) params.set(key, value)
  return apiResponse<{ rooms: Room[]; limit: number; offset: number }>(`/rooms?${params}`, { headers: { Authorization: `Bearer ${token}` } })
}

export function getRoom(token: string, id: string) {
  return apiResponse<RoomDetail>(`/rooms/${id}`, { headers: { Authorization: `Bearer ${token}` } }).then((detail) => ({ ...detail, players: detail.players ?? [], live: detail.live ?? { available: false, reason: 'unavailable' } }))
}
