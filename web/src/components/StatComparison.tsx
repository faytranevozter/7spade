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

// StatComparison is a visual "You vs them" duel on public profiles: scoreboard
// header, then dual-bar metric rows with per-stat deltas.
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

  const scored = rows.filter((row) => deltaTone(row) !== 'none')
  const myWins = scored.filter((row) => deltaTone(row) === 'win').length
  const theirWins = scored.filter((row) => deltaTone(row) === 'lose').length
  const ties = scored.filter((row) => deltaTone(row) === 'even').length
  const edge =
    myWins > theirWins ? 'ahead' : myWins < theirWins ? 'behind' : scored.length > 0 ? 'tied' : 'even'

  return (
    <section
      className="overflow-hidden rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] shadow-spade-card"
      aria-label={`Your stats compared with ${opponentName}`}
    >
      <div className="border-b border-spade-cream/10 bg-spade-bg/35 px-4 py-4 sm:px-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">
              Head to head
            </h3>
            <p className="mt-1 text-sm text-spade-gray-3">All-time stats · not games played together</p>
          </div>
          <EdgeBadge edge={edge} myWins={myWins} theirWins={theirWins} ties={ties} />
        </div>

        <div className="mt-4 grid grid-cols-[1fr_auto_1fr] items-center gap-3">
          <PlayerSide label="You" name="You" value={`${myWins}`} tone="you" align="left" />
          <div className="grid place-items-center">
            <span className="rounded-full border border-spade-gold/35 bg-spade-gold/15 px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-spade-gold-light">
              VS
            </span>
          </div>
          <PlayerSide
            label={opponentName}
            name={opponentName}
            value={`${theirWins}`}
            tone="them"
            align="right"
          />
        </div>
      </div>

      <ul className="grid gap-0 divide-y divide-spade-cream/8 px-3 py-1 sm:px-4">
        {rows.map((row) => (
          <ComparisonRow key={row.label} row={row} />
        ))}
      </ul>
    </section>
  )
}

function PlayerSide({
  label,
  name,
  value,
  tone,
  align,
}: {
  label: string
  name: string
  value: string
  tone: 'you' | 'them'
  align: 'left' | 'right'
}) {
  return (
    <div className={align === 'right' ? 'text-right' : 'text-left'}>
      <p
        className={`truncate text-sm font-medium ${
          tone === 'you' ? 'text-spade-gold-light' : 'text-spade-cream'
        }`}
        title={name}
      >
        {label}
      </p>
      <p className="mt-0.5 font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gray-3">
        categories led
      </p>
      <p
        className={`mt-1 text-3xl font-semibold tabular-nums ${
          tone === 'you' ? 'text-spade-gold-light' : 'text-spade-cream'
        }`}
      >
        {value}
      </p>
    </div>
  )
}

function EdgeBadge({
  edge,
  myWins,
  theirWins,
  ties,
}: {
  edge: 'ahead' | 'behind' | 'tied' | 'even'
  myWins: number
  theirWins: number
  ties: number
}) {
  const copy =
    edge === 'ahead'
      ? 'You lead'
      : edge === 'behind'
        ? 'They lead'
        : edge === 'tied'
          ? 'Dead even'
          : 'No comparison'

  const className =
    edge === 'ahead'
      ? 'border-green-400/30 bg-green-400/10 text-green-400'
      : edge === 'behind'
        ? 'border-spade-red/30 bg-spade-red/10 text-spade-red'
        : 'border-spade-cream/15 bg-spade-bg/45 text-spade-gray-2'

  return (
    <div className={`rounded-spade-md border px-3 py-2 text-right ${className}`}>
      <p className="text-sm font-medium">{copy}</p>
      <p className="mt-0.5 font-mono text-[10px] uppercase tracking-[0.08em] opacity-80">
        {myWins}–{theirWins}
        {ties > 0 ? ` · ${ties} tied` : ''}
      </p>
    </div>
  )
}

function ComparisonRow({ row }: { row: Row }) {
  const tone = deltaTone(row)
  const delta = formatDelta(row)
  const { minePct, theirsPct } = barPercents(row)

  return (
    <li className="py-3">
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <span className="text-sm text-spade-gray-2">{row.label}</span>
        {tone !== 'none' ? (
          <span
            className={`rounded-spade-pill border px-2 py-0.5 font-mono text-[11px] ${
              tone === 'win'
                ? 'border-green-400/30 bg-green-400/10 text-green-400'
                : tone === 'lose'
                  ? 'border-spade-red/30 bg-spade-red/10 text-spade-red'
                  : 'border-spade-cream/12 bg-spade-bg/40 text-spade-gray-3'
            }`}
          >
            {delta}
          </span>
        ) : (
          <span className="font-mono text-[11px] text-spade-gray-3">—</span>
        )}
      </div>

      <div className="grid grid-cols-[minmax(3rem,auto)_1fr_minmax(3rem,auto)] items-center gap-2">
        <span
          className={`text-right font-mono text-sm font-semibold tabular-nums ${
            tone === 'win' ? 'text-spade-gold-light' : 'text-spade-cream'
          }`}
        >
          {row.mineText}
        </span>

        <div className="grid h-2 grid-cols-2 overflow-hidden rounded-full bg-spade-cream/8" aria-hidden="true">
          <div className="flex justify-end bg-transparent">
            <div
              className={`h-full rounded-l-full ${
                tone === 'win' ? 'bg-spade-gold' : tone === 'lose' ? 'bg-spade-cream/25' : 'bg-spade-cream/35'
              }`}
              style={{ width: `${minePct}%` }}
            />
          </div>
          <div className="flex justify-start bg-transparent">
            <div
              className={`h-full rounded-r-full ${
                tone === 'lose' ? 'bg-spade-red/80' : tone === 'win' ? 'bg-spade-cream/25' : 'bg-spade-cream/35'
              }`}
              style={{ width: `${theirsPct}%` }}
            />
          </div>
        </div>

        <span className="font-mono text-sm tabular-nums text-spade-gray-2">{row.theirsText}</span>
      </div>

      <div className="mt-1 grid grid-cols-2 gap-2 font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
        <span>You</span>
        <span className="text-right">Them</span>
      </div>
    </li>
  )
}

type Tone = 'win' | 'lose' | 'even' | 'none'

function deltaTone(row: Row): Tone {
  if (row.better === 'neutral') return 'none'
  if (row.mineValue === null || row.theirsValue === null) return 'none'
  if (row.mineValue === row.theirsValue) return 'even'
  const mineIsHigher = row.mineValue > row.theirsValue
  const mineIsBetter = row.better === 'higher' ? mineIsHigher : !mineIsHigher
  return mineIsBetter ? 'win' : 'lose'
}

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

function barPercents(row: Row): { minePct: number; theirsPct: number } {
  if (row.mineValue === null || row.theirsValue === null) {
    return { minePct: 0, theirsPct: 0 }
  }

  // Invert for lower-is-better so the visual still reads as "stronger side longer".
  let mine = Math.abs(row.mineValue)
  let theirs = Math.abs(row.theirsValue)
  if (row.better === 'lower') {
    const peak = Math.max(mine, theirs, 0.0001)
    mine = peak - mine
    theirs = peak - theirs
  }

  const peak = Math.max(mine, theirs, 0.0001)
  // Each half of the duel bar fills relative to the stronger side (100% = lead).
  return {
    minePct: Math.max(mine > 0 ? 10 : 0, Math.round((mine / peak) * 100)),
    theirsPct: Math.max(theirs > 0 ? 10 : 0, Math.round((theirs / peak) * 100)),
  }
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

function top2Rate(stats: UserStatsDto): number {
  return (stats.first_place_count + stats.second_place_count) / stats.games_played
}
