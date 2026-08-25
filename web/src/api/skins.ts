import { apiRequest } from './client'

export type SkinType = 'profile_background' | 'avatar_frame' | 'display_picture' | 'player_card_background'

export type SkinDto = {
  id: string
  skin_type: SkinType
  name: string
  description: string
  asset_key: string
  display_order: number
}

export type CatalogSkinDto = SkinDto & {
  unlock_requirement?: string
}

export type OwnedSkinDto = SkinDto & {
  source: string
  equipped: boolean
}

export type EquippedSkinDto = {
  skin_type: SkinType
  skin_id: string
  asset_key: string
}

export type UserSkinsResponse = {
  owned: OwnedSkinDto[]
  equipped: EquippedSkinDto[]
}

export function getSkinCatalog(token: string | null): Promise<{ skins: CatalogSkinDto[] }> {
  return apiRequest<{ skins: CatalogSkinDto[] }>('/skins', { token })
}

export function getMySkins(token: string | null): Promise<UserSkinsResponse> {
  return apiRequest<UserSkinsResponse>('/me/skins', { token })
}

export function getUserSkins(token: string | null, userID: string): Promise<UserSkinsResponse> {
  return apiRequest<UserSkinsResponse>(`/users/${encodeURIComponent(userID)}/skins`, { token })
}

export function equipSkin(token: string | null, skinType: SkinType, skinID: string): Promise<UserSkinsResponse> {
  return apiRequest<UserSkinsResponse>(`/me/skins/${skinType}`, {
    method: 'PUT',
    token,
    body: { skin_id: skinID },
  })
}

export function unequipSkin(token: string | null, skinType: SkinType): Promise<UserSkinsResponse> {
  return apiRequest<UserSkinsResponse>(`/me/skins/${skinType}`, { method: 'DELETE', token })
}

export function skinAssetURL(assetKey: string): string | null {
  const base = import.meta.env.VITE_SKIN_ASSETS_URL?.replace(/\/$/, '')
  return base && assetKey ? `${base}/${assetKey}` : null
}

export function equippedAssetKey(skins: EquippedSkinDto[], skinType: SkinType): string | undefined {
  return skins.find((skin) => skin.skin_type === skinType)?.asset_key
}

export function equippedSkin(skins: EquippedSkinDto[], skinType: SkinType): EquippedSkinDto | undefined {
  return skins.find((skin) => skin.skin_type === skinType)
}

export function skinUnlockSourceLabel(source: string): string {
  if (source.startsWith('achievement:')) {
    return `Achievement reward: ${titleCase(source.slice('achievement:'.length))}`
  }
  if (source.startsWith('level:')) return `Level ${source.slice('level:'.length)} reward`
  if (source.startsWith('login_streak:')) return `${source.slice('login_streak:'.length)}-day login streak reward`
  if (source.startsWith('game_condition:')) return 'Completed-game challenge reward'
  if (source.startsWith('backfill:')) return 'Progression reward'
  return 'New cosmetic unlocked'
}

function titleCase(value: string): string {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}
