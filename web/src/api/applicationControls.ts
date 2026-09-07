import { apiRequest } from './client'

export type ApplicationControls = {
  new_registrations: boolean
  guest_access: boolean
  room_creation: boolean
  quick_play: boolean
  new_game_starts: boolean
  spectator_access: boolean
  emotes: boolean
}

export function getApplicationControls(): Promise<ApplicationControls> {
  return apiRequest<ApplicationControls>('/application-controls')
}
