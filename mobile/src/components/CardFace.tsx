import { Pressable, Text, View, type ViewStyle } from 'react-native'
import { suitColor, suitSymbols } from '../game/cards'
import type { Card } from '../types'

type CardSize = 'sm' | 'md' | 'board'

type CardFaceProps = {
  card: Card
  size?: CardSize
  interactive?: boolean
  onPress?: () => void
  accessibilityLabel?: string
}

// Native port of web/src/components/CardFace.tsx. Renders a playing card on a
// light surface with corner pips and a centre glyph. Selected/playable/dimmed
// states match the web visuals (lift + ring -> border + translate).
const dimensions: Record<CardSize, { width: number; height: number }> = {
  sm: { width: 52, height: 76 },
  md: { width: 70, height: 100 },
  board: { width: 26, height: 37 },
}

export function CardFace({
  card,
  size = 'md',
  interactive = true,
  onPress,
  accessibilityLabel,
}: CardFaceProps) {
  const color = suitColor[card.suit]
  const label = `${card.rank} of ${card.suit}`
  const dim = dimensions[size]
  const isBoard = size === 'board'

  const containerStyle: ViewStyle = {
    width: dim.width,
    height: dim.height,
    transform: [{ translateY: card.selected ? -12 : 0 }],
  }

  const borderClass = card.selected
    ? 'border-2 border-spade-gold'
    : card.playable
      ? 'border-2 border-spade-green-light'
      : 'border border-black/10'

  const opacityClass = card.dimmed ? 'opacity-45' : ''

  const cornerFont = isBoard ? 7 : size === 'sm' ? 9 : 12
  const centreFont = isBoard ? 12 : size === 'sm' ? 18 : 24

  const content = (
    <View
      className={`relative rounded-spade-card bg-spade-white ${borderClass} ${opacityClass}`}
      style={containerStyle}
    >
      <Text style={{ position: 'absolute', left: 3, top: 1, color, fontSize: cornerFont, fontWeight: '700' }}>
        {card.rank}
      </Text>
      <View className="flex-1 items-center justify-center">
        <Text style={{ color, fontSize: centreFont }}>{suitSymbols[card.suit]}</Text>
      </View>
      {!isBoard ? (
        <Text style={{ position: 'absolute', right: 3, bottom: 1, color, fontSize: cornerFont, fontWeight: '700', transform: [{ rotate: '180deg' }] }}>
          {card.rank}
        </Text>
      ) : null}
    </View>
  )

  if (!interactive || !onPress) {
    return (
      <View accessibilityLabel={accessibilityLabel ?? label} accessible>
        {content}
      </View>
    )
  }

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? (card.playable ? `Play ${label}` : label)}
      onPress={onPress}
      className="active:opacity-80"
    >
      {content}
    </Pressable>
  )
}

export function FaceDownCard({ size = 'md' }: { size?: CardSize }) {
  const dim = dimensions[size]
  return (
    <View
      accessibilityLabel="Face-down card"
      accessibilityRole="image"
      className="rounded-spade-card bg-spade-green"
      style={{ width: dim.width, height: dim.height }}
    />
  )
}
