import type { RatingEventDto } from '../api/stats'

type RatingHistoryProps = {
  // Events are newest-first (as returned by the API).
  events: RatingEventDto[]
}

const SPARK_WIDTH = 280
const SPARK_HEIGHT = 56
const SPARK_PAD = 4

// RatingHistory shows a player's rating trajectory: a compact inline sparkline
// of rating_after over time plus the most recent per-game deltas. No charting
// dependency — the line is a hand-built SVG polyline. Renders nothing when there
// are no events, so the caller can mount it unconditionally.
export function RatingHistory({ events }: RatingHistoryProps) {
  if (events.length === 0) return null

  // API returns newest-first; plot oldest -> newest left-to-right.
  const ordered = [...events].reverse()
  const ratings = ordered.map((e) => e.rating_after)
  const min = Math.min(...ratings)
  const max = Math.max(...ratings)
  const span = max - min || 1

  const points = ordered
    .map((e, i) => {
      const x = ordered.length === 1
        ? SPARK_WIDTH / 2
        : SPARK_PAD + (i / (ordered.length - 1)) * (SPARK_WIDTH - SPARK_PAD * 2)
      const y = SPARK_PAD + (1 - (e.rating_after - min) / span) * (SPARK_HEIGHT - SPARK_PAD * 2)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')

  const latest = events[0]
  // Most recent few games (newest-first) as signed deltas.
  const recent = events.slice(0, 8)

  return (
    <section
      className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-4"
      aria-label="Rating history"
    >
      <div className="mb-3 flex items-baseline justify-between">
        <h3 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">Rating history</h3>
        <span className="font-mono text-xs text-spade-gray-3">
          now {latest.rating_after}
        </span>
      </div>

      <svg
        viewBox={`0 0 ${SPARK_WIDTH} ${SPARK_HEIGHT}`}
        className="h-14 w-full"
        preserveAspectRatio="none"
        role="img"
        aria-label={`Rating from ${min} to ${max} over ${ordered.length} games`}
      >
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinejoin="round"
          strokeLinecap="round"
          className="text-spade-gold-light"
        />
      </svg>

      <ul className="mt-3 flex flex-wrap gap-2">
        {recent.map((event) => (
          <li
            key={event.game_id}
            className={`rounded-spade-md border px-2 py-1 font-mono text-xs ${deltaClass(event.rating_delta)}`}
          >
            {formatDelta(event.rating_delta)}
          </li>
        ))}
      </ul>
    </section>
  )
}

function formatDelta(delta: number): string {
  if (delta > 0) return `+${delta}`
  return String(delta)
}

function deltaClass(delta: number): string {
  if (delta > 0) return 'border-green-400/40 text-green-400'
  if (delta < 0) return 'border-spade-red/40 text-spade-red'
  return 'border-spade-cream/12 text-spade-gray-3'
}
