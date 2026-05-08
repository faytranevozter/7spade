import { apiRequest } from './client'

export type RoomVisibility = 'public' | 'private'

export type RoomDto = {
  id: string
  invite_code: string
  player_count: number
  turn_timer_seconds: number
  status?: string
  visibility?: RoomVisibility
  name?: string
}

export type CreateRoomRequest = {
  visibility: RoomVisibility
  turn_timer_seconds: number
}

export type CreateRoomResponse = {
  id: string
  invite_code: string
}

export type JoinRoomResponse = {
  id: string
}

export function getRooms(token: string | null): Promise<RoomDto[]> {
  return apiRequest<RoomDto[]>('/rooms', { token })
}

export function postRoom(token: string | null, body: CreateRoomRequest): Promise<CreateRoomResponse> {
  return apiRequest<CreateRoomResponse>('/rooms', {
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
