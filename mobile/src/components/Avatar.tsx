import { useState } from 'react'
import { Image, Text, View } from 'react-native'

type AvatarProps = {
  avatarUrl?: string | null
  initials: string
  tone?: 'green' | 'gold' | 'dark' | 'red'
  // Pixel diameter of the circle. Defaults to 36.
  size?: number
  alt?: string
}

// Native port of web/src/components/Avatar.tsx. Renders a player's OAuth photo
// with a graceful initials fallback (broken/missing URLs fall back to the tone
// circle). Tracks the failed URL so a refreshed avatar is retried.
const toneClasses: Record<NonNullable<AvatarProps['tone']>, string> = {
  green: 'bg-spade-green-mid',
  gold: 'bg-[#7a5010]',
  dark: 'bg-[#2a2a3a]',
  red: 'bg-[#922b21]',
}

export function Avatar({ avatarUrl, initials, tone = 'green', size = 36, alt }: AvatarProps) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const showImage = Boolean(avatarUrl) && avatarUrl !== failedUrl
  const dimension = { width: size, height: size, borderRadius: size / 2 }

  if (showImage) {
    return (
      <Image
        source={{ uri: avatarUrl as string }}
        accessibilityLabel={alt ?? initials}
        onError={() => setFailedUrl(avatarUrl as string)}
        style={dimension}
      />
    )
  }

  return (
    <View
      accessibilityLabel={alt ?? initials}
      className={`items-center justify-center ${toneClasses[tone]}`}
      style={dimension}
    >
      <Text className="text-sm font-medium text-spade-cream">{initials}</Text>
    </View>
  )
}
