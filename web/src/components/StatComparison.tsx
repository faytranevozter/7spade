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
  // Raw numeric values for delta math; null when not applicable (e.g. 0 games).
  mineValue: number | null
  theirsValue: number | null
  // Display strings (already formatted).
  mineText: string
  theirsText: string
  // Which direction is "better" — drives the delta coloring.
  better: Direction
  // Delta is shown as a percentage-point value (win rate) vs a plain number.
  deltaKind: 'number' | 'points'
}

// StatComparison renders the viewer's lifetime stats side-by-side with another
// player's, with a per-metric delta from the viewer's perspective. It compares
// all-time stats only (not games played together). Lower-is-better metrics
// (avg penalty, best round) invert the delta coloring.
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
  ]

  return (
    <section className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-4" aria-label={`Your stats compared with ${opponentName}`}>
      <h3 className="mb-3 font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">You vs {opponentName}</h3>
      <div className="grid grid-cols-[1fr_auto_auto] gap-x-4 gap-y-2 text-sm">
        <span className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3" />
        <span className="text-right font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">You</span>
        <span className="max-w-[8rem] truncate text-right font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{opponentName}</span>
        {rows.map((row) => (
          <ComparisonRow key={row.label} row={row} />
        ))}
      </div>
    </section>
  )
}

function ComparisonRow({ row }: { row: Row }) {
  const tone = deltaTone(row)
  return (
    <>
      <span className="text-spade-gray-2">{row.label}</span>
      <span className={`text-right font-mono ${tone === 'win' ? 'text-spade-gold-light' : 'text-spade-cream'}`}>
        {row.mineText}
        {tone !== 'none' ? (
          <span className={`ml-1 text-[10px] ${tone === 'win' ? 'text-green-400' : tone === 'lose' ? 'text-spade-red' : 'text-spade-gray-3'}`}>
            {formatDelta(row)}
          </span>
        ) : null}
      </span>
      <span className="text-right font-mono text-spade-gray-2">{row.theirsText}</span>
    </>
  )
}

type Tone = 'win' | 'lose' | 'even' | 'none'

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
