import { View, Text } from 'react-native'

type BadgeTone = 'waiting' | 'playing' | 'passed' | 'winner' | 'danger'

// Native port of web/src/components/Badge.tsx. The leading dot is a small View
// (the web `before:` pseudo-element has no RN equivalent).
const containerClasses: Record<BadgeTone, string> = {
  waiting: 'border-spade-gold/30 bg-spade-gold/10',
  playing: 'border-spade-green-light/30 bg-spade-green-light/10',
  passed: 'border-spade-gray-3/30 bg-spade-gray-3/10',
  winner: 'border-spade-gold/45 bg-spade-gold/20',
  danger: 'border-spade-red/35 bg-spade-red/10',
}

const textClasses: Record<BadgeTone, string> = {
  waiting: 'text-spade-gold-light',
  playing: 'text-[#7bd696]',
  passed: 'text-spade-gray-2',
  winner: 'text-spade-gold-light',
  danger: 'text-[#ffb4a8]',
}

const dotClasses: Record<BadgeTone, string> = {
  waiting: 'bg-spade-gold',
  playing: 'bg-spade-green-light',
  passed: 'bg-spade-gray-3',
  winner: 'bg-spade-gold-light',
  danger: 'bg-spade-red',
}

export function Badge({ children, tone = 'waiting' }: { children: string; tone?: BadgeTone }) {
  return (
    <View className={`flex-row items-center gap-1.5 self-start rounded-spade-pill border px-3 py-1 ${containerClasses[tone]}`}>
      <View className={`size-1.5 rounded-full ${dotClasses[tone]}`} />
      <Text className={`text-[11px] font-medium ${textClasses[tone]}`}>{children}</Text>
    </View>
  )
}
