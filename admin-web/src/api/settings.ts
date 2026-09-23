import { apiResponse, csrfHeaders } from './client'

export type SettingType = 'boolean' | 'integer' | 'float' | 'string' | 'options'

export type FeatureSetting = {
  key: string
  type: SettingType
  value: boolean | number | string | unknown[] | Record<string, unknown>
  updated_at?: string
}

export function getApplicationSettings(token: string) {
  return apiResponse<FeatureSetting[]>('/settings', {
    headers: { Authorization: `Bearer ${token}` },
  })
}

export function updateApplicationSetting(
  token: string,
  key: string,
  value: FeatureSetting['value'],
  reason: string,
) {
  return apiResponse<FeatureSetting>(`/settings/${key}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...csrfHeaders(),
    },
    body: JSON.stringify({ value, reason }),
  })
}
