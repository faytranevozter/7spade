import { useEffect, useState } from 'react'
import { getUserSkins, type EquippedSkinDto } from '../api/skins'
import { Avatar } from './Avatar'

type SkinnedAvatarProps = React.ComponentProps<typeof Avatar> & {
  userId?: string
}

const skinCache = new Map<string, EquippedSkinDto[]>()
const pending = new Map<string, Promise<EquippedSkinDto[]>>()

function loadSkins(userId: string): Promise<EquippedSkinDto[]> {
  const cached = skinCache.get(userId)
  if (cached) return Promise.resolve(cached)
  const inFlight = pending.get(userId)
  if (inFlight) return inFlight

  const request = getUserSkins(null, userId)
    .then((response) => {
      skinCache.set(userId, response.equipped)
      return response.equipped
    })
    .catch(() => [])
    .finally(() => pending.delete(userId))
  pending.set(userId, request)
  return request
}

// SkinnedAvatar resolves public equipped cosmetics once per user per page session.
// It keeps list and live-game payloads compact while avoiding JWT skin claims.
export function SkinnedAvatar({ userId, ...avatarProps }: SkinnedAvatarProps) {
  const [skins, setSkins] = useState<EquippedSkinDto[]>(() => (userId ? skinCache.get(userId) ?? [] : []))
  const frameSkin = skins.find((skin) => skin.skin_type === 'avatar_frame')
  const displayPictureSkin = skins.find((skin) => skin.skin_type === 'display_picture')

  useEffect(() => {
    let cancelled = false
    if (!userId) {
      setSkins([])
      return undefined
    }
    void loadSkins(userId).then((next) => {
      if (!cancelled) setSkins(next)
    })
    return () => {
      cancelled = true
    }
  }, [userId])

  return (
    <Avatar
      {...avatarProps}
      frameAssetKey={frameSkin?.asset_key}
      frameSkinId={frameSkin?.skin_id}
      displayPictureAssetKey={displayPictureSkin?.asset_key}
      displayPictureSkinId={displayPictureSkin?.skin_id}
    />
  )
}
