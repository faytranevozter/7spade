import { apiBlob, apiResponse } from './client'

export type AuditEvent = {
  id: string
  actor_id?: string
  session_id?: string
  request_id?: string
  action: string
  resource_type?: string
  resource_id?: string
  reason?: string
  outcome: string
  before_state?: unknown
  after_state?: unknown
  metadata?: unknown
  ip_address?: string
  user_agent?: string
  occurred_at: string
}

export type AuditFilters = {
  id?: string
  actor_id?: string
  action?: string
  resource_type?: string
  resource_id?: string
  outcome?: string
  from?: string
  to?: string
}

export type AuditEventPage = {
  events: AuditEvent[]
  limit: number
  offset: number
}

function auditParams(filters: AuditFilters, limit?: number, offset?: number) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filters))
    if (value) params.set(key, value)
  if (limit !== undefined) params.set('limit', String(limit))
  if (offset !== undefined) params.set('offset', String(offset))
  return params
}

export function getAuditEvents(
  token: string,
  filters: AuditFilters,
  limit = 50,
  offset = 0,
) {
  return apiResponse<AuditEventPage>(
    `/audit-events?${auditParams(filters, limit, offset)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  )
}

export function getAuditEvent(token: string, id: string) {
  return apiResponse<{ events: AuditEvent[] }>(
    `/audit-events?id=${encodeURIComponent(id)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  ).then((page) => page.events[0])
}

export function exportAuditEvents(token: string, filters: AuditFilters) {
  return apiBlob(`/audit-events/export?${auditParams(filters)}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
}
