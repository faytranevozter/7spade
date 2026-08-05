import { useState } from 'react'
import { useSkinAsset } from '../hooks/useSkinAsset'

type AvatarProps = {
  // The avatar image URL; when absent or it fails to load, the initials circle
  // is shown instead.
  avatarUrl?: string | null
  // Fallback initials (already computed by the caller).
  initials: string
  // Tone background for the initials fallback circle.
  tone?: 'green' | 'gold' | 'dark' | 'red'
  // Tailwind size class for the circle, e.g. "size-9". Defaults to size-9.
  sizeClass?: string
  // Extra classes for the outer circle (e.g. ring/border state).
  className?: string
  // Accessible label for the image; defaults to the initials.
  alt?: string
  // S3-backed cosmetic asset keys. A display picture takes precedence over an
  // OAuth avatar; frames are purely decorative overlays.
  displayPictureAssetKey?: string
  displayPictureSkinId?: string
  frameAssetKey?: string
  frameSkinId?: string
}

const toneClasses: Record<NonNullable<AvatarProps['tone']>, string> = {
  green: 'bg-spade-green-mid',
  gold: 'bg-[#7a5010]',
  dark: 'bg-[#2a2a3a]',
  red: 'bg-[#922b21]',
}

// Avatar renders a player's OAuth photo with a graceful initials fallback.
// Guests, bots, email/password users, and broken image URLs all fall back to
// the tone + initials circle. referrerPolicy avoids leaking the app URL to the
// third-party image host.
export function Avatar({
  avatarUrl,
  initials,
  tone = 'green',
  sizeClass = 'size-9',
  className = '',
  alt,
  displayPictureAssetKey,
  displayPictureSkinId,
  frameAssetKey,
  frameSkinId,
}: AvatarProps) {
  // Track which URL failed to load rather than a boolean, so a new avatarUrl is
  // retried automatically (a reused positional instance won't stay stuck on the
  // initials fallback after a prior URL failed, e.g. a seat whose player
  // reconnects with a refreshed avatar).
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const [failedFrameURL, setFailedFrameURL] = useState<string | null>(null)
  const displayPictureURL = useSkinAsset(displayPictureSkinId, displayPictureAssetKey)
  const imageURL = displayPictureURL ?? avatarUrl ?? null
  const frameURL = useSkinAsset(frameSkinId, frameAssetKey)
  const showImage = Boolean(imageURL) && imageURL !== failedUrl

  return (
    <span
      className={`relative inline-grid ${sizeClass}`}
    >
      {showImage ? (
        <img
          src={imageURL as string}
          alt={alt ?? initials}
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={() => setFailedUrl(imageURL as string)}
          className={`size-full rounded-full object-cover ${className}`}
        />
      ) : (
        <span
          className={`grid size-full place-items-center rounded-full ${toneClasses[tone]} text-sm font-medium text-spade-cream ${className}`}
          aria-label={alt ?? initials}
        >
          {initials}
        </span>
      )}
      {frameURL && frameURL !== failedFrameURL ? (
        <img
          src={frameURL}
          alt=""
          aria-hidden="true"
          loading="lazy"
          onError={() => setFailedFrameURL(frameURL)}
          className="pointer-events-none absolute inset-0 size-full object-contain"
        />
      ) : null}
    </span>
  )
}
