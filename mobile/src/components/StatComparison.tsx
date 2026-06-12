import { Text, View } from 'react-native'
import type { UserStatsDto } from '../api/stats'

type StatComparisonProps = {
  // The viewing player's own stats ("You").
  mine: UserStatsDto
  // The profile being viewed.
  theirs: UserStatsDto
  opponentName: string
}

type Direction = 'higher' | 'lower' | 'neutral'

type Row = {
  label: string
  mineValue: number | null
  theirsValue: number | null
  mineText: string
  theirsText: string
  better: Direction
  deltaKind: 'number' | 'points'
}

type Tone = 'win' | 'lose' | 'even' | 'none'

// Native port of web/src/components/StatComparison.tsx. Shows the viewer's
// lifetime stats side-by-side with another player's, with per-metric deltas from
// the viewer's perspective. All-time stats only (not games played together);
// lower-is-better metrics invert the delta coloring.
export function StatComparison({ mine, theirs, opponentName }: StatComparisonProps) {
  const rows: Row[] = [
    {
      label: 'Rating',
      mineValue: mine.rating,
      theirsValue: theirs.rating,
      mineText: String(mine.rating),
      theirsText: String(theirs.rating),
      better: 'higher',
      deltaKind: 'number',
    },
    {
      label: 'Games played',
      mineValue: mine.games_played,
      theirsValue: theirs.games_played,
      mineText: String(mine.games_played),
      theirsText: String(theirs.games_played),
      better: 'neutral',
      deltaKind: 'number',
    },
    {
      label: 'Wins',
      mineValue: mine.wins,
      theirsValue: theirs.wins,
      mineText: String(mine.wins),
      theirsText: String(theirs.wins),
      better: 'higher',
      deltaKind: 'number',
    },
    {
      label: 'Win rate',
      mineValue: mine.games_played > 0 ? mine.win_rate : null,
      theirsValue: theirs.games_played > 0 ? theirs.win_rate : null,
      mineText: mine.games_played > 0 ? formatPercent(mine.win_rate) : '—',
      theirsText: theirs.games_played > 0 ? formatPercent(theirs.win_rate) : '—',
      better: 'higher',
      deltaKind: 'points',
    },
    {
      label: 'Avg penalty',
      mineValue: mine.games_played > 0 ? mine.avg_penalty : null,
      theirsValue: theirs.games_played > 0 ? theirs.avg_penalty : null,
      mineText: mine.games_played > 0 ? mine.avg_penalty.toFixed(1) : '—',
      theirsText: theirs.games_played > 0 ? theirs.avg_penalty.toFixed(1) : '—',
      better: 'lower',
      deltaKind: 'number',
    },
    {
      label: 'Best round',
      mineValue: mine.best_penalty,
      theirsValue: theirs.best_penalty,
      mineText: mine.best_penalty === null ? '—' : String(mine.best_penalty),
      theirsText: theirs.best_penalty === null ? '—' : String(theirs.best_penalty),
      better: 'lower',
      deltaKind: 'number',
    },
    {
      label: 'Avg rank',
      mineValue: mine.games_played > 0 ? mine.avg_rank : null,
      theirsValue: theirs.games_played > 0 ? theirs.avg_rank : null,
      mineText: mine.games_played > 0 ? mine.avg_rank.toFixed(2) : '—',
      theirsText: theirs.games_played > 0 ? theirs.avg_rank.toFixed(2) : '—',
      better: 'lower',
      deltaKind: 'number',
    },
    {
      label: 'Top 2 rate',
      mineValue: mine.games_played > 0 ? top2Rate(mine) : null,
      theirsValue: theirs.games_played > 0 ? top2Rate(theirs) : null,
      mineText: mine.games_played > 0 ? formatPercent(top2Rate(mine)) : '—',
      theirsText: theirs.games_played > 0 ? formatPercent(top2Rate(theirs)) : '—',
      better: 'higher',
      deltaKind: 'points',
    },
    {
      label: 'Best win streak',
      mineValue: mine.best_win_streak,
      theirsValue: theirs.best_win_streak,
      mineText: String(mine.best_win_streak),
      theirsText: String(theirs.best_win_streak),
      better: 'higher',
      deltaKind: 'number',
    },
  ]

  return (
    <View className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-4">
      <Text className="mb-3 font-mono text-[11px] uppercase tracking-wider text-spade-gold">You vs {opponentName}</Text>
      <View className="flex-row border-b border-spade-cream/10 pb-1">
        <Text className="flex-1 font-mono text-[10px] uppercase text-spade-gray-3" />
        <Text className="w-24 text-right font-mono text-[10px] uppercase text-spade-gray-3">You</Text>
        <Text className="w-20 text-right font-mono text-[10px] uppercase text-spade-gray-3" numberOfLines={1}>{opponentName}</Text>
      </View>
      {rows.map((row) => (
        <ComparisonRow key={row.label} row={row} />
      ))}
    </View>
  )
}

function ComparisonRow({ row }: { row: Row }) {
  const tone = deltaTone(row)
  const deltaColor = tone === 'win' ? 'text-green-400' : tone === 'lose' ? 'text-spade-red' : 'text-spade-gray-3'
  return (
    <View className="flex-row items-center border-t border-spade-cream/10 py-2">
      <Text className="flex-1 text-sm text-spade-gray-2">{row.label}</Text>
      <View className="w-24 flex-row items-baseline justify-end gap-1">
        <Text className={`font-mono text-sm ${tone === 'win' ? 'text-spade-gold-light' : 'text-spade-cream'}`}>{row.mineText}</Text>
        {tone !== 'none' ? <Text className={`font-mono text-[10px] ${deltaColor}`}>{formatDelta(row)}</Text> : null}
      </View>
      <Text className="w-20 text-right font-mono text-sm text-spade-gray-2">{row.theirsText}</Text>
    </View>
  )
}

// deltaTone decides the coloring of the viewer's value: win (green) when the
// viewer is better on a directional metric, lose (red) when worse, even when
// equal, none for neutral metrics or when a value is unavailable.
function deltaTone(row: Row): Tone {
  if (row.better === 'neutral') return 'none'
  if (row.mineValue === null || row.theirsValue === null) return 'none'
  if (row.mineValue === row.theirsValue) return 'even'
  const mineIsHigher = row.mineValue > row.theirsValue
  const mineIsBetter = row.better === 'higher' ? mineIsHigher : !mineIsHigher
  return mineIsBetter ? 'win' : 'lose'
}

// formatDelta renders the signed difference (mine − theirs) from the viewer's
// perspective. Win-rate deltas are expressed in percentage points.
function formatDelta(row: Row): string {
  if (row.mineValue === null || row.theirsValue === null) return ''
  const diff = row.mineValue - row.theirsValue
  if (diff === 0) return '='
  if (row.deltaKind === 'points') {
    const pts = diff * 100
    return `${pts > 0 ? '+' : ''}${pts.toFixed(1)}pp`
  }
  const rounded = Number.isInteger(diff) ? String(diff) : diff.toFixed(1)
  return `${diff > 0 ? '+' : ''}${rounded}`
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

// top2Rate is the share of games finished 1st or 2nd. Caller guards games > 0.
function top2Rate(stats: UserStatsDto): number {
  return (stats.first_place_count + stats.second_place_count) / stats.games_played
}
