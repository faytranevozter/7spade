import { Text, View } from 'react-native'
import { Badge } from './Badge'
import type { Score } from '../types'

// Native port of web/src/components/ScoreTable.tsx. Rebuilt as a flex grid since
// React Native has no <table>. Columns: #, Player, Cards left, Penalty, Result.
export function ScoreTable({ scores, winnerLabel = 'Winner' }: { scores: Score[]; winnerLabel?: string }) {
  return (
    <View className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
      <View className="flex-row border-b border-spade-cream/10 bg-spade-cream/5 px-3 py-2">
        <Text className="w-8 font-mono text-[10px] uppercase text-spade-gray-3">#</Text>
        <Text className="flex-1 font-mono text-[10px] uppercase text-spade-gray-3">Player</Text>
        <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Cards</Text>
        <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Penalty</Text>
        <Text className="w-20 text-right font-mono text-[10px] uppercase text-spade-gray-3">Result</Text>
      </View>
      {scores.map((score) => (
        <View
          key={score.player}
          className={`flex-row items-center border-t border-spade-cream/10 px-3 py-3 ${score.me ? 'bg-spade-gold/10' : ''}`}
        >
          <Text className="w-8 font-medium text-spade-gold">{score.rank}</Text>
          <Text className="flex-1 text-sm text-spade-cream" numberOfLines={1}>{score.player}</Text>
          <Text className="w-14 text-right font-mono text-sm text-spade-cream">{score.cardsLeft}</Text>
          <Text className="w-14 text-right font-mono text-sm text-spade-cream">{score.penalty}</Text>
          <View className="w-20 items-end">
            {score.winner ? (
              <Badge tone="winner">{winnerLabel}</Badge>
            ) : (
              <Text className="text-xs text-spade-gray-2">{score.result}</Text>
            )}
          </View>
        </View>
      ))}
    </View>
  )
}
