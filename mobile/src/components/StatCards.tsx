import { Text, View } from 'react-native'
import type { UserStatsDto } from '../api/stats'

type StatCardsProps = {
  stats: UserStatsDto
}

// Native port of web/src/components/StatCards.tsx. A read-only grid of a
// registered player's lifetime stats.
export function StatCards({ stats }: StatCardsProps) {
  const cards: { label: string; value: string }[] = [
    { label: 'Rating', value: String(stats.rating) },
    { label: 'Games played', value: String(stats.games_played) },
    { label: 'Wins', value: String(stats.wins) },
    { label: 'Win rate', value: formatPercent(stats.win_rate) },
    { label: 'Avg penalty', value: stats.games_played > 0 ? stats.avg_penalty.toFixed(1) : '—' },
    { label: 'Best round', value: stats.best_penalty === null ? '—' : String(stats.best_penalty) },
    { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—' },
  ]

  return (
    <View className="flex-row flex-wrap gap-3">
      {cards.map((card) => (
        <View
          key={card.label}
          className="min-w-[30%] flex-1 rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-4 py-3"
        >
          <Text className="font-mono text-[10px] uppercase text-spade-gray-3">{card.label}</Text>
          <Text className="mt-1 text-2xl font-medium text-spade-cream">{card.value}</Text>
        </View>
      ))}
    </View>
  )
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}
