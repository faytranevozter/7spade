import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { ApiError } from '../api/client'
import { getEvents, type EventStatus, type EventSummary } from '../api/events'
import { SceneShell } from '../components/SceneShell'

type EventFilter = 'all' | EventStatus

const filters: Array<{ value: EventFilter; label: string }> = [
  { value: 'all', label: 'All events' },
  { value: 'active', label: 'Active' },
  { value: 'upcoming', label: 'Upcoming' },
  { value: 'ended', label: 'Ended' },
]

const statusLabels: Record<EventStatus, string> = {
  active: 'Live now',
  upcoming: 'Coming soon',
  ended: 'Completed',
}

export function EventsPage() {
  const [events, setEvents] = useState<EventSummary[]>([])
  const [filter, setFilter] = useState<EventFilter>('all')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    getEvents()
      .then((response) => {
        if (!cancelled) setEvents(response.events)
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : 'Failed to load events')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  const visibleEvents = filter === 'all' ? events : events.filter((event) => event.status === filter)

  return (
    <SceneShell title="Events" eyebrow="Limited-time tables">
      <div className="grid gap-6 pb-4">
        <div className="grid gap-4 border-b border-spade-cream/10 pb-5 lg:grid-cols-[1fr_auto] lg:items-end">
          <div className="max-w-2xl">
            <p className="text-lg leading-relaxed text-spade-gray-2">Join seasonal challenges, check in daily, and unlock rewards that only appear while an event is at the table.</p>
          </div>
          <div className="flex flex-wrap gap-2" role="group" aria-label="Filter events">
            {filters.map((item) => (
              <button
                key={item.value}
                type="button"
                aria-pressed={filter === item.value}
                onClick={() => setFilter(item.value)}
                className={`min-h-10 rounded-spade-pill border px-4 py-2 text-sm font-medium transition ${filter === item.value ? 'border-spade-gold-light bg-spade-gold text-spade-bg' : 'border-spade-cream/10 bg-spade-bg/50 text-spade-gray-2 hover:border-spade-gold/45 hover:text-spade-cream'}`}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>

        {loading ? <EventGridSkeleton /> : null}
        {!loading && error ? <p role="alert" className="rounded-spade-lg border border-spade-red/35 bg-spade-red/10 p-5 text-spade-cream">{error}</p> : null}
        {!loading && !error && visibleEvents.length === 0 ? (
          <div className="rounded-spade-xl border border-dashed border-spade-gold/25 bg-spade-bg/35 px-6 py-14 text-center">
            <p className="font-mono text-xs uppercase tracking-[0.2em] text-spade-gold-light">Quiet table</p>
            <h2 className="mt-3 text-2xl font-medium text-spade-cream">No {filter === 'all' ? '' : `${filter} `}events found</h2>
            <p className="mx-auto mt-2 max-w-lg text-spade-gray-2">Check back when the next limited-time challenge is announced.</p>
          </div>
        ) : null}
        {!loading && !error && visibleEvents.length > 0 ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {visibleEvents.map((event) => <EventCard key={event.id} event={event} />)}
          </div>
        ) : null}
      </div>
    </SceneShell>
  )
}

function EventCard({ event }: { event: EventSummary }) {
  const accent = event.accent_color ?? '#d4af37'
  return (
    <article className="group relative isolate flex min-h-80 overflow-hidden rounded-spade-xl border border-spade-cream/10 bg-spade-bg shadow-spade-card transition hover:-translate-y-1 hover:border-spade-gold/35">
      <div className="absolute inset-0 opacity-35" style={{ background: `radial-gradient(circle at 85% 10%, ${accent}, transparent 42%)` }} />
      <div className="absolute inset-y-0 left-0 w-1" style={{ backgroundColor: accent }} />
      <div className="relative flex w-full flex-col p-6">
        <div className="flex items-start justify-between gap-3">
          <span className={`rounded-spade-pill border px-3 py-1 font-mono text-[10px] font-medium uppercase tracking-[0.16em] ${event.status === 'active' ? 'border-spade-gold/45 bg-spade-gold/15 text-spade-gold-light' : 'border-spade-cream/15 bg-spade-green/30 text-spade-gray-2'}`}>{statusLabels[event.status]}</span>
          <span className="font-mono text-[10px] uppercase tracking-wider text-spade-gray-3">{event.reward_count} {event.reward_count === 1 ? 'reward' : 'rewards'}</span>
        </div>
        <div className="mt-auto pt-16">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-spade-gold-light">{formatDateRange(event.starts_at, event.ends_at)}</p>
          <h2 className="mt-3 text-2xl font-medium tracking-tight text-spade-cream">{event.name}</h2>
          <p className="mt-2 line-clamp-3 text-sm leading-relaxed text-spade-gray-2">{event.summary}</p>
          <Link to={`/events/${event.slug}`} className="mt-6 inline-flex min-h-10 items-center gap-2 rounded-spade-md border border-spade-gold/35 bg-spade-green/40 px-4 py-2 text-sm font-medium text-spade-cream transition group-hover:border-spade-gold group-hover:bg-spade-gold group-hover:text-spade-bg">
            View event <span aria-hidden="true">&rarr;</span>
          </Link>
        </div>
      </div>
    </article>
  )
}

function EventGridSkeleton() {
  return <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Loading events">
    {[0, 1, 2].map((item) => <div key={item} className="h-80 animate-pulse rounded-spade-xl border border-spade-cream/10 bg-spade-bg/65" />)}
  </div>
}

function formatDateRange(startsAt: string, endsAt: string): string {
  const formatter = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  return `${formatter.format(new Date(startsAt))} - ${formatter.format(new Date(endsAt))}`
}
