import { useEquippedSkinsState } from '../hooks/useEquippedSkins'
import type { EquippedSkinDto } from '../api/skins'
import { Avatar } from './Avatar'

type SkinnedAvatarProps = React.ComponentProps<typeof Avatar> & {
  userId?: string
  equippedSkins?: EquippedSkinDto[]
}

// SkinnedAvatar resolves public equipped cosmetics once per user per page session.
// It keeps list and live-game payloads compact while avoiding JWT skin claims.
export function SkinnedAvatar({ userId, equippedSkins, ...avatarProps }: SkinnedAvatarProps) {
  const { skins, isLoading } = useEquippedSkinsState(equippedSkins === undefined ? userId : undefined)
  const resolvedSkins = equippedSkins ?? skins
  const frameSkin = resolvedSkins.find((skin) => skin.skin_type === 'avatar_frame')
  const displayPictureSkin = resolvedSkins.find((skin) => skin.skin_type === 'display_picture')

  return (
    <Avatar
      {...avatarProps}
      frameAssetKey={frameSkin?.asset_key}
      frameSkinId={frameSkin?.skin_id}
      displayPictureAssetKey={displayPictureSkin?.asset_key}
      displayPictureSkinId={displayPictureSkin?.skin_id}
      displayPictureLoading={equippedSkins ? false : isLoading}
    />
  )
}
