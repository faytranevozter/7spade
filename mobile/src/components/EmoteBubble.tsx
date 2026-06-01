import { Text, View } from 'react-native'
import { emoteGlyph } from '../game/emotes'
import type { ActiveEmote } from '../hooks/useGameSocket'

// Native port of web/src/components/EmoteBubble.tsx. Renders the active emote
// for a seat as a floating bubble above its anchor. The parent positions it
// (absolute) — here we just render the pill.
export function EmoteBubble({ emote }: { emote: ActiveEmote | undefined }) {
  if (!emote) return null
  const glyph = emoteGlyph(emote.id)
  if (!glyph) return null

  const isWord = /[a-zA-Z]/.test(glyph)

  return (
    <View
      key={emote.seq}
      accessibilityRole="text"
      accessibilityLabel={`Emote: ${glyph}`}
      className="absolute -top-9 left-1/2 z-10 -translate-x-1/2"
    >
      <View className="rounded-spade-pill border border-spade-gold/40 bg-spade-bg/95 px-2.5 py-1">
        <Text className={isWord ? 'text-sm font-semibold text-spade-gold-light' : 'text-xl leading-none'}>
          {glyph}
        </Text>
      </View>
    </View>
  )
}
