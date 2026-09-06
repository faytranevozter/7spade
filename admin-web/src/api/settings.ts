import { apiResponse, csrfHeaders } from './client'

export type FeatureSetting = {
  key: string
  enabled: boolean
}

export function getDailyLoginSetting(token: string) {
  return apiResponse<FeatureSetting>('/settings/daily-login', {
    headers: { Authorization: `Bearer ${token}` },
  })
}

export function updateDailyLoginSetting(
  token: string,
  enabled: boolean,
  reason: string,
) {
  return apiResponse<FeatureSetting>('/settings/daily-login', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...csrfHeaders(),
    },
    body: JSON.stringify({ enabled, reason }),
  })
}
