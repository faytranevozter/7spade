import { apiResponse } from './client'

export type AuditEvent = {
  id: string
  action: string
  resource_type?: string
  resource_id?: string
  reason?: string
  outcome: string
  occurred_at: string
}

export function getAuditEvent(token: string, id: string) {
  return apiResponse<{ events: AuditEvent[] }>(`/audit-events?id=${encodeURIComponent(id)}`, { headers: { Authorization: `Bearer ${token}` } })
    .then((page) => page.events[0])
}
