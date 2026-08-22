import { useEquippedSkins } from '../hooks/useEquippedSkins'
import { Avatar } from './Avatar'

type SkinnedAvatarProps = React.ComponentProps<typeof Avatar> & {
  userId?: string
}

// SkinnedAvatar resolves public equipped cosmetics once per user per page session.
// It keeps list and live-game payloads compact while avoiding JWT skin claims.
export function SkinnedAvatar({ userId, ...avatarProps }: SkinnedAvatarProps) {
  const skins = useEquippedSkins(userId)
  const frameSkin = skins.find((skin) => skin.skin_type === 'avatar_frame')
  const displayPictureSkin = skins.find((skin) => skin.skin_type === 'display_picture')

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
