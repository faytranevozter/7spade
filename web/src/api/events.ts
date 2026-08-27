import { apiRequest } from './client'
import type { CatalogSkinDto, SkinDto } from './skins'

export type EventStatus = 'upcoming' | 'active' | 'ended'

export type EventSummary = {
  id: string
  slug: string
  name: string
  summary: string
  starts_at: string
  ends_at: string
  app_timezone: string
  hero_asset_key?: string
  accent_color?: string
  status: EventStatus
  server_time: string
  reward_count: number
}

export type EventListResponse = {
  events: EventSummary[]
}

export type EventDetail = {
  event: Omit<EventSummary, 'reward_count'> & {
    description: string
  }
  check_in: {
    authenticated: boolean
    count: number
    claimed_today: boolean
    next_claim_at?: string
  }
  skin_rewards: Array<{
    skin: CatalogSkinDto
    requirement: {
      type: 'achievement' | 'game_condition' | 'minimum_level' | 'login_streak' | 'event_check_in_count'
      description: string
      progress?: number
      target?: number
      completed: boolean
    }
    owned: boolean
  }>
}

export type EventClaimResult = {
  newly_claimed: boolean
  check_in: EventDetail['check_in']
  skin_grants: Array<SkinDto & { source: string }>
}

export function getEvents(): Promise<EventListResponse> {
  return apiRequest<EventListResponse>('/events')
}

export function getEvent(token: string | null, slug: string): Promise<EventDetail> {
  return apiRequest<EventDetail>(`/events/${encodeURIComponent(slug)}`, { token })
}

export function claimEventCheckIn(token: string | null, slug: string): Promise<EventClaimResult> {
  return apiRequest<EventClaimResult>(`/events/${encodeURIComponent(slug)}/check-ins`, { method: 'POST', token })
}
