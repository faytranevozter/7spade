import { apiRequest } from './client'
import type { SkinDto } from './skins'

export type EventStatus = 'upcoming' | 'active' | 'ended'

export type EventDetail = {
  event: {
    id: string
    slug: string
    name: string
    summary: string
    description: string
    starts_at: string
    ends_at: string
    timezone: string
    hero_asset_key?: string
    accent_color?: string
    status: EventStatus
    server_time: string
  }
  check_in: {
    authenticated: boolean
    count: number
    claimed_today: boolean
    next_claim_at?: string
  }
  skin_rewards: Array<{
    skin: SkinDto
    requirement: string
    target?: number
    progress: number
    completed: boolean
    owned: boolean
  }>
}

export type EventClaimResult = {
  newly_claimed: boolean
  check_in: EventDetail['check_in']
  skin_grants: Array<SkinDto & { source: string }>
}

export function getEvent(token: string | null, slug: string): Promise<EventDetail> {
  return apiRequest<EventDetail>(`/events/${encodeURIComponent(slug)}`, { token })
}

export function claimEventCheckIn(token: string | null, slug: string): Promise<EventClaimResult> {
  return apiRequest<EventClaimResult>(`/events/${encodeURIComponent(slug)}/check-ins`, { method: 'POST', token })
}
