import { apiResponse, csrfHeaders } from './client'

export type FeatureSetting = {
  key: string
  enabled: boolean
}

export function getApplicationSettings(token: string) {
  return apiResponse<FeatureSetting[]>('/settings', {
    headers: { Authorization: `Bearer ${token}` },
  })
}

export function updateApplicationSetting(
  token: string,
  key: string,
  enabled: boolean,
  reason: string,
) {
  return apiResponse<FeatureSetting>(`/settings/${key}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...csrfHeaders(),
    },
    body: JSON.stringify({ enabled, reason }),
  })
}
