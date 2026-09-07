import { apiResponse } from './client'
import type { Skin } from './skins'

export type EventState = 'draft' | 'scheduled' | 'published' | 'archived'
export type EventRewardConfig = Record<string, unknown> & {
  daily_login?: {
    enabled: boolean
    xp_per_claim: number
  }
}
export type AdminEvent = {
  id: string
  slug: string
  name: string
  summary: string
  description: string
  starts_at: string
  ends_at: string
  hero_asset_key?: string
  accent_color?: string
  reward_config: EventRewardConfig
  state: EventState
  revision: number
  version: number
  published_at?: string
  archived_at?: string
}
export type EventInput = Omit<
  AdminEvent,
  'id' | 'state' | 'revision' | 'published_at' | 'archived_at'
> & { reason: string }
const headers = (token: string) => ({
  'Content-Type': 'application/json',
  Authorization: `Bearer ${token}`,
})
export const getEvents = (token: string) =>
  apiResponse<{ events: AdminEvent[] }>('/events', {
    headers: { Authorization: `Bearer ${token}` },
  })
export const getEvent = (token: string, id: string) =>
  apiResponse<{ event: AdminEvent; skin_rewards: Skin[] }>(
    `/events/${id}?preview=true`,
    {
      headers: { Authorization: `Bearer ${token}` },
    },
  )
export const createEvent = (token: string, event: EventInput) =>
  apiResponse<AdminEvent>('/events', {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify(event),
  })
export const updateEvent = (token: string, id: string, event: EventInput) =>
  apiResponse<AdminEvent>(`/events/${id}`, {
    method: 'PUT',
    headers: headers(token),
    body: JSON.stringify(event),
  })
export const transitionEvent = (
  token: string,
  id: string,
  action: 'schedule' | 'publish' | 'archive',
  version: number,
  reason: string,
) =>
  apiResponse<AdminEvent>(`/events/${id}/${action}`, {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({ version, reason }),
  })
