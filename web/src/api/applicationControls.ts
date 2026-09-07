import { apiRequest } from './client'

export type ApplicationControls = {
  new_registrations: boolean
  guest_access: boolean
  room_creation: boolean
  quick_play: boolean
}

export function getApplicationControls(): Promise<ApplicationControls> {
  return apiRequest<ApplicationControls>('/application-controls')
}
