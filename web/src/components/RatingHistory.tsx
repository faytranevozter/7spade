import { useEffect, useId, useMemo, useRef, useState } from 'react'
import type { RatingEventDto } from '../api/stats'

type RatingHistoryProps = {
  // Events are newest-first (as returned by the API).
  events: RatingEventDto[]
}

const CHART_H = 200
const DEFAULT_CHART_W = 720
const PAD = { top: 16, right: 16, bottom: 28, left: 44 }

type ChartPoint = {
  event: RatingEventDto
  x: number
  y: number
  index: number
}

// RatingHistory shows a player's rating trajectory: area chart of rating_after
// over time, summary strip, and recent per-game deltas. No charting dependency —
// pure SVG. Renders nothing when there are no events. Chart width tracks the
// container so the plot always spans full width.
export function RatingHistory({ events }: RatingHistoryProps) {
  const gradientId = useId()
  const chartRef = useRef<HTMLDivElement>(null)
  const [chartW, setChartW] = useState(DEFAULT_CHART_W)
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)

  useEffect(() => {
    const el = chartRef.current
    if (!el) return

    const update = (width: number) => {
      const next = Math.max(280, Math.round(width))
      setChartW((prev) => (prev === next ? prev : next))
    }

    update(el.clientWidth)
    if (typeof ResizeObserver === 'undefined') return

    const ro = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width
      if (width) update(width)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const chart = useMemo(() => buildChart(events, chartW), [events, chartW])
  if (!chart) return null

  const { ordered, points, min, max, yTicks, pathLine, pathArea, latest, oldest, netDelta } = chart
  const hover = hoverIndex !== null ? points[hoverIndex] : null
  const recent = events.slice(0, 12)

  return (
    <section
      className="w-full rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-4 sm:p-5"
      aria-label="Rating history"
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">
            Rating history
          </h3>
          <p className="mt-0.5 text-xs text-spade-gray-3">
            {ordered.length} rated game{ordered.length === 1 ? '' : 's'}
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <StatChip label="Now" value={String(latest.rating_after)} />
          <StatChip
            label="Net"
            value={formatDelta(netDelta)}
            tone={netDelta > 0 ? 'up' : netDelta < 0 ? 'down' : 'neutral'}
          />
          <StatChip label="Peak" value={String(max)} />
          <StatChip label="Low" value={String(min)} />
        </div>
      </div>

      <div
        ref={chartRef}
        className="relative w-full rounded-spade-md border border-spade-cream/8 bg-spade-bg/40 px-1 pt-2 pb-1"
      >
        <svg
          width="100%"
          height={CHART_H}
          viewBox={`0 0 ${chartW} ${CHART_H}`}
          className="block w-full"
          preserveAspectRatio="none"
          role="img"
          aria-label={`Rating from ${min} to ${max} over ${ordered.length} games`}
          onMouseLeave={() => setHoverIndex(null)}
        >
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#f5c842" stopOpacity="0.35" />
              <stop offset="100%" stopColor="#f5c842" stopOpacity="0" />
            </linearGradient>
          </defs>

          {yTicks.map((tick, i) => (
            <g key={`grid-${tick.value}-${i}`}>
              <line
                x1={PAD.left}
                y1={tick.y}
                x2={chartW - PAD.right}
                y2={tick.y}
                stroke="currentColor"
                strokeWidth="1"
                className="text-spade-cream/10"
                strokeDasharray={i === 1 ? '4 4' : undefined}
              />
              <text
                x={PAD.left - 8}
                y={tick.y + 3}
                textAnchor="end"
                className="fill-spade-gray-3"
                fontSize="10"
                fontFamily="ui-monospace, monospace"
              >
                {tick.value}
              </text>
            </g>
          ))}

          <path d={pathArea} fill={`url(#${gradientId})`} />
          <path
            d={pathLine}
            fill="none"
            stroke="#f5c842"
            strokeWidth="2.25"
            strokeLinejoin="round"
            strokeLinecap="round"
          />

          {hover ? (
            <line
              x1={hover.x}
              y1={PAD.top}
              x2={hover.x}
              y2={CHART_H - PAD.bottom}
              stroke="currentColor"
              strokeWidth="1"
              className="text-spade-cream/25"
              strokeDasharray="3 3"
            />
          ) : null}

          {points.map((p) => {
            const active = hoverIndex === p.index
            const up = p.event.rating_delta > 0
            const down = p.event.rating_delta < 0
            return (
              <g key={p.event.game_id}>
                <circle
                  cx={p.x}
                  cy={p.y}
                  r={active ? 14 : 10}
                  fill="transparent"
                  className="cursor-pointer"
                  onMouseEnter={() => setHoverIndex(p.index)}
                  onFocus={() => setHoverIndex(p.index)}
                  tabIndex={0}
                  role="button"
                  aria-label={`Game rating ${p.event.rating_after}, ${formatDelta(p.event.rating_delta)}`}
                />
                <circle
                  cx={p.x}
                  cy={p.y}
                  r={active ? 5 : 3.5}
                  fill={up ? '#4ade80' : down ? '#f87171' : '#f5c842'}
                  stroke="#102316"
                  strokeWidth="1.5"
                  className="pointer-events-none"
                />
              </g>
            )
          })}

          <text
            x={PAD.left}
            y={CHART_H - 8}
            className="fill-spade-gray-3"
            fontSize="10"
            fontFamily="ui-monospace, monospace"
          >
            {formatShortDate(oldest.created_at)}
          </text>
          <text
            x={chartW - PAD.right}
            y={CHART_H - 8}
            textAnchor="end"
            className="fill-spade-gray-3"
            fontSize="10"
            fontFamily="ui-monospace, monospace"
          >
            {formatShortDate(latest.created_at)}
          </text>
        </svg>

        {hover ? (
          <div
            className="pointer-events-none absolute top-3 left-1/2 z-10 -translate-x-1/2 rounded-spade-md border border-spade-cream/15 bg-[#1a241c] px-3 py-2 shadow-spade-card"
            role="status"
          >
            <p className="font-mono text-sm font-semibold text-spade-cream">
              {hover.event.rating_after}{' '}
              <span className={deltaTextClass(hover.event.rating_delta)}>
                ({formatDelta(hover.event.rating_delta)})
              </span>
            </p>
            <p className="mt-0.5 font-mono text-[10px] text-spade-gray-3">
              {formatLongDate(hover.event.created_at)}
            </p>
          </div>
        ) : null}
      </div>

      <div className="mt-4">
        <h4 className="mb-2 font-mono text-[10px] uppercase tracking-[0.1em] text-spade-gray-3">
          Recent games
        </h4>
        <ul className="grid gap-1.5">
          {recent.map((event) => (
            <li
              key={event.game_id}
              className="flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/8 bg-spade-bg/35 px-3 py-2"
            >
              <span className="font-mono text-[11px] text-spade-gray-3">
                {formatLongDate(event.created_at)}
              </span>
              <div className="flex items-center gap-3 font-mono text-xs">
                <span className={deltaTextClass(event.rating_delta)}>
                  {formatDelta(event.rating_delta)}
                </span>
                <span className="text-spade-cream">{event.rating_after}</span>
              </div>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}

function buildChart(events: RatingEventDto[], chartW: number) {
  if (events.length === 0) return null

  const ordered = [...events].reverse()
  const ratings = ordered.map((e) => e.rating_after)
  const rawMin = Math.min(...ratings)
  const rawMax = Math.max(...ratings)
  // Pad the range slightly so flat lines don't hug the edges.
  const pad = Math.max(8, Math.round((rawMax - rawMin) * 0.12) || 8)
  const scaleMin = rawMin - pad
  const scaleMax = rawMax + pad
  const span = scaleMax - scaleMin || 1
  const plotW = chartW - PAD.left - PAD.right
  const plotH = CHART_H - PAD.top - PAD.bottom

  const toY = (rating: number) => PAD.top + (1 - (rating - scaleMin) / span) * plotH

  const points: ChartPoint[] = ordered.map((event, i) => {
    const x =
      ordered.length === 1
        ? PAD.left + plotW / 2
        : PAD.left + (i / (ordered.length - 1)) * plotW
    return { event, x, y: toY(event.rating_after), index: i }
  })

  const pathLine =
    points.length === 1
      ? `M ${points[0].x} ${points[0].y}`
      : points
          .map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
          .join(' ')

  const baseline = CHART_H - PAD.bottom
  const pathArea =
    points.length === 0
      ? ''
      : `${pathLine} L ${points[points.length - 1].x.toFixed(1)} ${baseline} L ${points[0].x.toFixed(1)} ${baseline} Z`

  const tickValues = [rawMax, Math.round((rawMin + rawMax) / 2), rawMin]
  const yTicks = tickValues.map((value) => ({ value, y: toY(value) }))

  const latest = events[0]
  const oldest = ordered[0]
  const netDelta = latest.rating_after - oldest.rating_before

  return {
    ordered,
    points,
    min: rawMin,
    max: rawMax,
    yTicks,
    pathLine,
    pathArea,
    latest,
    oldest,
    netDelta,
  }
}

function StatChip({
  label,
  value,
  tone = 'neutral',
}: {
  label: string
  value: string
  tone?: 'up' | 'down' | 'neutral'
}) {
  const valueClass =
    tone === 'up'
      ? 'text-green-400'
      : tone === 'down'
        ? 'text-red-400'
        : 'text-spade-cream'

  return (
    <div className="min-w-[4.5rem] rounded-spade-md border border-spade-cream/10 bg-spade-bg/40 px-2.5 py-1.5 text-center">
      <p className="font-mono text-[9px] uppercase tracking-[0.08em] text-spade-gray-3">{label}</p>
      <p className={`font-mono text-sm font-semibold ${valueClass}`}>{value}</p>
    </div>
  )
}

function formatDelta(delta: number): string {
  if (delta > 0) return `+${delta}`
  return String(delta)
}

function deltaTextClass(delta: number): string {
  if (delta > 0) return 'text-green-400'
  if (delta < 0) return 'text-red-400'
  return 'text-spade-gray-3'
}

function formatShortDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function formatLongDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}
