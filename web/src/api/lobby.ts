import { apiRequest } from './client'

export type RoomVisibility = 'public' | 'private'
export type RoomStatus = 'waiting' | 'in_progress' | 'finished'
export type BotDifficulty = 'easy' | 'medium' | 'hard'

export type RoomDto = {
  id: string
  invite_code: string
  visibility: RoomVisibility
  turn_timer_seconds: number
  bot_difficulty: BotDifficulty
  practice_mode: boolean
  min_elo: number | null
  max_elo: number | null
  status: RoomStatus
  player_count: number
}

export type CreateRoomRequest = {
  visibility: RoomVisibility
  turn_timer_seconds: number
  bot_difficulty: BotDifficulty
  practice_mode?: boolean
  min_elo?: number
  max_elo?: number
}

export type QuickPlayRequest = {
  ranked?: boolean
}

export type JoinRoomResponse = {
  id: string
  invite_code: string
  status: RoomStatus
  player_count: number
}

export type ActiveRoomDto = {
  id: string
  invite_code: string
  status: RoomStatus
  practice_mode: boolean
}

export function getRooms(token: string | null): Promise<RoomDto[]> {
  return apiRequest<RoomDto[]>('/rooms', { token })
}

export function getRoom(token: string | null, id: string): Promise<RoomDto> {
  return apiRequest<RoomDto>(`/rooms/${encodeURIComponent(id)}`, { token })
}

export function postRoom(token: string | null, body: CreateRoomRequest): Promise<RoomDto> {
  return apiRequest<RoomDto>('/rooms', {
    method: 'POST',
    token,
    body,
  })
}

export function postJoinRoom(token: string | null, inviteCode: string): Promise<JoinRoomResponse> {
  return apiRequest<JoinRoomResponse>(`/rooms/${encodeURIComponent(inviteCode)}/join`, {
    method: 'POST',
    token,
  })
}

export function postQuickPlay(token: string | null, body?: QuickPlayRequest): Promise<JoinRoomResponse> {
  return apiRequest<JoinRoomResponse>('/rooms/quick-play', {
    method: 'POST',
    token,
    body,
  })
}

export function getMyActiveRoom(token: string | null): Promise<{ active_room: ActiveRoomDto | null }> {
  return apiRequest<{ active_room: ActiveRoomDto | null }>('/my/active-room', { token })
}
