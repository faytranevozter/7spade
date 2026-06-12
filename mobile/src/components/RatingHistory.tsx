import { Text, View } from 'react-native'
import type { RatingEventDto } from '../api/stats'

type RatingHistoryProps = {
  // Events are newest-first (as returned by the API).
  events: RatingEventDto[]
}

// Native port of web/src/components/RatingHistory.tsx. Without an SVG charting
// dependency, the trajectory is shown as thin proportional bars of rating_after
// plus the most recent per-game deltas. Renders nothing when there are no
// events, so the caller can mount it unconditionally.
export function RatingHistory({ events }: RatingHistoryProps) {
  if (events.length === 0) return null

  // API returns newest-first; show bars oldest -> newest left-to-right.
  const ordered = [...events].reverse()
  const ratings = ordered.map((e) => e.rating_after)
  const min = Math.min(...ratings)
  const max = Math.max(...ratings)
  const span = max - min || 1

  const latest = events[0]
  const recent = events.slice(0, 8)

  return (
    <View className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-4">
      <View className="mb-3 flex-row items-baseline justify-between">
        <Text className="font-mono text-[11px] uppercase tracking-wider text-spade-gold">Rating history</Text>
        <Text className="font-mono text-xs text-spade-gray-3">now {latest.rating_after}</Text>
      </View>

      <View className="h-14 flex-row items-end gap-[2px]">
        {ordered.map((event, i) => {
          const heightPct = 20 + ((event.rating_after - min) / span) * 80
          return (
            <View
              key={`${event.game_id}-${i}`}
              className="flex-1 rounded-t-sm bg-spade-gold-light"
              style={{ height: `${heightPct}%` }}
            />
          )
        })}
      </View>

      <View className="mt-3 flex-row flex-wrap gap-2">
        {recent.map((event) => (
          <View
            key={event.game_id}
            className={`rounded-spade-md border px-2 py-1 ${deltaClass(event.rating_delta)}`}
          >
            <Text className={`font-mono text-xs ${deltaTextClass(event.rating_delta)}`}>
              {formatDelta(event.rating_delta)}
            </Text>
          </View>
        ))}
      </View>
    </View>
  )
}

function formatDelta(delta: number): string {
  if (delta > 0) return `+${delta}`
  return String(delta)
}

function deltaClass(delta: number): string {
  if (delta > 0) return 'border-green-400/40'
  if (delta < 0) return 'border-spade-red/40'
  return 'border-spade-cream/12'
}

function deltaTextClass(delta: number): string {
  if (delta > 0) return 'text-green-400'
  if (delta < 0) return 'text-spade-red'
  return 'text-spade-gray-3'
}
