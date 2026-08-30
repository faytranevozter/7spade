import { apiResponse } from './client'
export type Achievement = {
  id: string
  name: string
}
export type Revision = {
  id: string
  version: number
  asset_key: string
  content_type: string
  enabled: boolean
}
export type SkinUnlockCondition = {
  metric: string
  operator: 'eq' | 'gte' | 'lte' | 'gt' | 'lt'
  value: string
}
export type SkinUnlockRule = {
  id?: string
  name: string
  rule_type:
    | 'achievement'
    | 'minimum_level'
    | 'login_streak'
    | 'event_check_in_count'
    | 'game_condition'
  achievement_id?: string
  minimum_level?: number
  login_streak_days?: number
  event_id?: string
  event_check_in_count?: number
  retroactive: boolean
  enabled: boolean
  conditions?: SkinUnlockCondition[]
}
export type Skin = {
  id: string
  skin_type: string
  name: string
  description: string
  asset_key: string
  asset_url: string
  is_starter: boolean
  display_order: number
  enabled: boolean
  catalog_visible: boolean
  unlock_rules_locked: boolean
  unlock_rules: SkinUnlockRule[]
  revisions: Revision[]
}
type SkinResponse = Omit<
  Skin,
  'asset_url' | 'is_starter' | 'unlock_rules' | 'revisions'
> & {
  asset_url?: string
  is_starter?: boolean
  unlock_rules: SkinUnlockRule[] | null
  revisions: Revision[] | null
}
const headers = (token: string) => ({
  'Content-Type': 'application/json',
  Authorization: `Bearer ${token}`,
})
const normalizeSkin = (skin: SkinResponse): Skin => ({
  ...skin,
  asset_url: skin.asset_url ?? '',
  is_starter: skin.is_starter ?? false,
  unlock_rules: skin.unlock_rules ?? [],
  revisions: skin.revisions ?? [],
})
export const skinTypeLabel = (value: string) =>
  value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
export const getSkins = (token: string) =>
  apiResponse<{ skins: SkinResponse[] | null }>('/skins', {
    headers: { Authorization: `Bearer ${token}` },
  }).then(({ skins }) => ({ skins: (skins ?? []).map(normalizeSkin) }))
export const getAchievements = (token: string) =>
  apiResponse<{ achievements: Achievement[] }>('/achievements', {
    headers: { Authorization: `Bearer ${token}` },
  })
export type CreateSkinInput = {
  name: string
  skin_type: string
  description: string
  display_order: number
  reason: string
}
export const createSkin = (token: string, input: CreateSkinInput) =>
  apiResponse<SkinResponse>('/skins', {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify(input),
  }).then(normalizeSkin)
export const saveSkin = (
  token: string,
  skin: Skin,
  reason: string,
  unlock_rules?: SkinUnlockRule[],
) => {
  const metadata = {
    id: skin.id,
    skin_type: skin.skin_type,
    name: skin.name,
    description: skin.description,
    asset_key: skin.asset_key,
    display_order: skin.display_order,
    enabled: skin.enabled,
    catalog_visible: skin.catalog_visible,
  }
  return apiResponse<SkinResponse>(`/skins/${skin.id}`, {
    method: 'PUT',
    headers: headers(token),
    body: JSON.stringify({
      ...metadata,
      ...(unlock_rules === undefined ? {} : { unlock_rules }),
      reason,
    }),
  }).then(normalizeSkin)
}
export const createUpload = (token: string, id: string, file: File) =>
  apiResponse<{
    asset_key: string
    upload_url: string
    preview_url: string
    headers: Record<string, string>
  }>(`/skins/${id}/uploads`, {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({
      filename: file.name,
      content_type: file.type,
      size: file.size,
    }),
  })
export const uploadSkinAsset = (token: string, id: string, file: File) => {
  const body = new FormData()
  body.append('file', file)
  return apiResponse<{
    asset_key: string
    preview_url: string
    content_type: string
  }>(`/skins/${id}/assets`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body,
  })
}
export const publishSkin = (
  token: string,
  id: string,
  asset_key: string,
  content_type: string,
  reason: string,
) =>
  apiResponse<Revision>(`/skins/${id}/revisions`, {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({ asset_key, content_type, reason }),
  })
export const disableRevision = (
  token: string,
  skinId: string,
  id: string,
  reason: string,
) =>
  apiResponse<void>(`/skins/${skinId}/revisions/${id}/disable`, {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({ reason }),
  })
export const changeEntitlement = (
  token: string,
  user: string,
  skin: string,
  action: 'grant' | 'revoke',
  revision_id: string,
  reason: string,
) =>
  apiResponse(`/users/${user}/skins/${skin}/${action}`, {
    method: 'POST',
    headers: headers(token),
    body: JSON.stringify({ revision_id, reason }),
  })
