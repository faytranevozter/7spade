import { Text, View } from 'react-native'
import { Badge } from './Badge'
import { Button } from './Button'
import type { Room } from '../types'

// Native port of web/src/components/RoomCard.tsx. Shows a public room with a
// seat-fill bar and a Join button.
export function RoomCard({ room, onJoin }: { room: Room; onJoin?: () => void }) {
  const seats = Array.from({ length: room.maxSeats }, (_, i) => i < room.filledSeats)

  return (
    <View className={`gap-3 rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4 ${room.open ? '' : 'opacity-60'}`}>
      <View className="flex-row items-start justify-between gap-2">
        <View className="flex-1">
          <Text className="text-sm font-medium text-spade-cream" numberOfLines={1}>{room.name}</Text>
          <Text className="mt-0.5 text-xs text-spade-gray-2">{room.status}</Text>
        </View>
        <Badge tone={room.open ? 'playing' : 'passed'}>{room.open ? 'Open' : 'Full'}</Badge>
      </View>

      <View className="flex-row items-center gap-1.5" accessibilityLabel={`${room.filledSeats} of ${room.maxSeats} seats filled`}>
        {seats.map((filled, i) => (
          <View key={i} className={`h-2 flex-1 rounded-full ${filled ? 'bg-spade-green-light' : 'bg-spade-cream/10'}`} />
        ))}
      </View>

      <View className="flex-row items-center justify-between gap-2">
        <Text className="rounded-spade-sm border border-spade-gold/30 bg-spade-gold/10 px-2 py-0.5 font-mono text-[11px] tracking-wider text-spade-gold-light">
          {room.code}
        </Text>
        <Text className="font-mono text-[11px] text-spade-gray-3">{room.players} · {room.timer} · Bots: {room.botDifficulty}</Text>
      </View>

      <Button variant={room.open ? 'primary' : 'secondary'} disabled={!room.open} onPress={onJoin}>
        {room.open ? 'Join' : 'Full'}
      </Button>
    </View>
  )
}
