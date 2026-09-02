import { useEffect, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  createEvent,
  getEvent,
  getEvents,
  transitionEvent,
  updateEvent,
  type AdminEvent,
  type EventState,
} from '../api/events'
import { useAuth } from '../hooks/useAuth'

const inputClass =
  'rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75'

const statusTone: Record<EventState, string> = {
  draft: 'border-admin-border-input bg-admin-surface-raised text-admin-muted',
  scheduled:
    'border-admin-accent-border bg-admin-accent-soft text-admin-accent-bright',
  published:
    'border-admin-success-border bg-admin-success-bg text-admin-success',
  archived: 'border-admin-border-input bg-admin-canvas text-admin-muted-subtle',
}

function StatusPill({ state }: { state: EventState }) {
  return (
    <span
      className={`text-admin-xs gap-admin-3 inline-flex items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone[state]}`}
    >
      {state}
    </span>
  )
}

function PageHeader({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string
  title: string
  children: ReactNode
}) {
  return (
    <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
      <div>
        <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
          {eyebrow}
        </p>
        <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-hero leading-[0.95] font-medium tracking-[-0.06em]">
          {title}
        </h1>
        <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
          Schedule and publish versioned player experiences without rewriting
          historical rewards.
        </p>
      </div>
      {children}
    </header>
  )
}

function Panel({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string
  title: string
  children: ReactNode
}) {
  return (
    <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
      <div className="mb-5">
        <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
          {eyebrow}
        </p>
        <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
          {title}
        </h2>
      </div>
      {children}
    </section>
  )
}

export function EventsPage() {
  const { token, admin } = useAuth()
  const [events, setEvents] = useState<AdminEvent[]>([])
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  useEffect(() => {
    if (token)
      getEvents(token)
        .then(({ events }) => setEvents(events))
        .catch((cause) =>
          setError(
            cause instanceof Error ? cause.message : 'Failed to load events',
          ),
        )
  }, [token])
  const pageSize = 12
  const totalPages = Math.max(1, Math.ceil(events.length / pageSize))
  const currentPage = Math.min(page, totalPages)
  const visibleEvents = events.slice(
    (currentPage - 1) * pageSize,
    currentPage * pageSize,
  )
  return (
    <section className="mx-auto w-full max-w-360">
      <PageHeader eyebrow="Live content" title="Events">
        {admin?.permissions.includes('events.manage') && (
          <Link
            to="/events/new"
            className="bg-admin-accent border-admin-accent-border rounded-admin-input px-admin-15 py-admin-11 text-admin-button-ink self-end border text-center font-bold no-underline"
          >
            Create event
          </Link>
        )}
      </PageHeader>
      {error && (
        <div
          role="alert"
          className="text-admin-danger border-admin-danger-border bg-admin-danger-bg mt-6 rounded-lg border px-4 py-3"
        >
          {error}
        </div>
      )}
      <div className="mt-6 grid grid-cols-2 gap-4 max-[1050px]:grid-cols-1">
        {visibleEvents.map((event) => (
          <Link
            key={event.id}
            to={`/events/${event.id}`}
            className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card hover:border-admin-accent-border-hover hover:bg-admin-surface-raised group border p-5 text-inherit no-underline transition"
          >
            <div className="flex items-start justify-between gap-4">
              <div className="border-admin-accent-border bg-admin-accent-soft text-admin-accent grid size-12 place-items-center rounded-lg border font-mono">
                {event.name.slice(0, 1).toUpperCase()}
              </div>
              <StatusPill state={event.state} />
            </div>
            <h2 className="text-admin-ink-strong text-admin-heading mt-5 mb-2">
              {event.name}
            </h2>
            <p className="text-admin-muted m-0 min-h-12 leading-normal">
              {event.summary}
            </p>
            <div className="border-admin-border-divider text-admin-muted-subtle text-admin-caption mt-5 flex items-center justify-between border-t pt-3">
              <span>{formatRange(event.starts_at, event.ends_at)}</span>
              <span className="text-admin-accent">Open record →</span>
            </div>
          </Link>
        ))}
      </div>
      {!error && events.length === 0 && (
        <div className="text-admin-muted rounded-admin-panel border-admin-border-input mt-6 grid min-h-70 place-items-center border border-dashed p-8 text-center">
          No event records yet. Create a draft to start planning a player
          experience.
        </div>
      )}
      {totalPages > 1 && (
        <CatalogPager
          page={currentPage}
          totalPages={totalPages}
          total={events.length}
          onPrevious={() => setPage(currentPage - 1)}
          onNext={() => setPage(currentPage + 1)}
        />
      )}
    </section>
  )
}

function CatalogPager({
  page,
  totalPages,
  total,
  onPrevious,
  onNext,
}: {
  page: number
  totalPages: number
  total: number
  onPrevious: () => void
  onNext: () => void
}) {
  return (
    <nav
      className="border-admin-border-divider mt-6 flex items-center justify-between gap-4 border-t pt-4 max-[500px]:flex-col"
      aria-label="Event catalog pages"
    >
      <span className="text-admin-muted text-admin-field">
        Showing {(page - 1) * 12 + 1}–{Math.min(page * 12, total)} of {total}{' '}
        events
      </span>
      <div className="flex gap-2">
        <button
          className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-2 font-semibold disabled:cursor-not-allowed disabled:opacity-45"
          type="button"
          disabled={page === 1}
          onClick={onPrevious}
        >
          Previous
        </button>
        <button
          className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-2 font-semibold disabled:cursor-not-allowed disabled:opacity-45"
          type="button"
          disabled={page === totalPages}
          onClick={onNext}
        >
          Next
        </button>
      </div>
    </nav>
  )
}

export function EventCreatePage() {
  return <EventEditor />
}

export function EventDetailPage() {
  const { id = '' } = useParams()
  const { token, admin } = useAuth()
  const [event, setEvent] = useState<AdminEvent | null>(null)
  const [reason, setReason] = useState('')
  const [notice, setNotice] = useState<{
    kind: 'error' | 'success'
    text: string
  } | null>(null)
  useEffect(() => {
    if (token)
      getEvent(token, id)
        .then(setEvent)
        .catch((cause) =>
          setNotice({
            kind: 'error',
            text:
              cause instanceof Error ? cause.message : 'Failed to load event',
          }),
        )
  }, [id, token])
  if (!event)
    return (
      <section className="mx-auto w-full max-w-360">
        <div className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center border border-dashed p-8">
          {notice?.text ?? 'Loading event details...'}
        </div>
      </section>
    )
  const canManage = admin?.permissions.includes('events.manage') ?? false
  const act = async (action: 'schedule' | 'publish' | 'archive') => {
    if (!token || !reason.trim()) {
      setNotice({ kind: 'error', text: 'A change reason is required' })
      return
    }
    try {
      const next = await transitionEvent(
        token,
        event.id,
        action,
        event.version,
        reason,
      )
      setEvent(next)
      setReason('')
      setNotice({
        kind: 'success',
        text: `Event ${action === 'archive' ? 'archived' : `${action}d`}`,
      })
    } catch (cause) {
      setNotice({
        kind: 'error',
        text: cause instanceof Error ? cause.message : 'Failed to update event',
      })
    }
  }
  return (
    <section className="mx-auto w-full max-w-360">
      <Link
        className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
        to="/events"
      >
        ← Back to events
      </Link>
      <header className="border-admin-border border-b pb-6">
        <div>
          <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
            Event record
          </p>
          <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-detail-hero leading-[0.95] font-medium tracking-[-0.06em]">
            {event.name}
          </h1>
          <p className="text-admin-muted-subtle text-admin-note m-0 font-mono">
            {event.slug} · revision {event.revision}
          </p>
        </div>
      </header>
      {notice && (
        <div
          role={notice.kind === 'error' ? 'alert' : 'status'}
          className={`text-admin-action my-4 rounded-lg border px-4 py-3 ${notice.kind === 'error' ? 'text-admin-danger border-admin-danger-border bg-admin-danger-bg' : 'text-admin-success border-admin-success-border bg-admin-success-bg'}`}
        >
          {notice.text}
        </div>
      )}
      <div className="gap-admin-17 mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,360px)] items-start max-[1050px]:grid-cols-1">
        <div className="grid gap-5">
          <Panel eyebrow="Player-facing" title="Experience preview">
            <div
              className="rounded-admin-preview border-admin-accent-border-faint bg-admin-surface-preview border p-6"
              style={{ borderTopColor: event.accent_color ?? undefined }}
            >
              <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                {formatRange(event.starts_at, event.ends_at)}
              </p>
              <p className="text-admin-ink-strong text-admin-detail-hero mt-5 mb-3 leading-[0.98] tracking-tighter">
                {event.name}
              </p>
              <p className="text-admin-ink-soft text-admin-preview m-0">
                {event.summary}
              </p>
              <p className="text-admin-muted mt-6 mb-0 max-w-150 leading-[1.65]">
                {event.description}
              </p>
            </div>
          </Panel>
          <Panel eyebrow="Versioned configuration" title="Reward payload">
            <pre className="border-admin-border-faint bg-admin-canvas text-admin-ink-soft overflow-auto rounded-lg border p-4 text-sm leading-[1.6]">
              {JSON.stringify(event.reward_config, null, 2)}
            </pre>
          </Panel>
        </div>
        <aside className="grid gap-5">
          <Panel eyebrow="Release control" title="Lifecycle">
            <div className="text-admin-muted text-admin-field grid gap-3">
              <div className="border-admin-border-divider flex justify-between border-b pb-3">
                <span>State</span>
                <StatusPill state={event.state} />
              </div>
              <div className="border-admin-border-divider flex justify-between border-b pb-3">
                <span>Resource version</span>
                <code className="text-admin-ink">{event.version}</code>
              </div>
              <div>
                <span className="text-admin-muted-subtle text-admin-caption block">
                  Window
                </span>
                <strong className="text-admin-ink text-admin-field mt-1 block">
                  {formatRange(event.starts_at, event.ends_at)}
                </strong>
              </div>
            </div>
            {canManage ? (
              <div className="mt-5 grid gap-3">
                <label className="gap-admin-5 text-admin-field text-admin-muted grid">
                  <span className="text-admin-label font-mono tracking-wider uppercase">
                    Change reason
                  </span>
                  <textarea
                    aria-label="Change reason"
                    className={`${inputClass} min-h-24 resize-y`}
                    value={reason}
                    placeholder="Why is this lifecycle change needed?"
                    onChange={(next) => setReason(next.target.value)}
                  />
                </label>
                <div className="grid gap-2">
                  {event.state === 'draft' && (
                    <button
                      className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-3 font-semibold disabled:opacity-50"
                      disabled={!reason.trim()}
                      onClick={() => void act('schedule')}
                    >
                      Schedule
                    </button>
                  )}
                  {(event.state === 'draft' || event.state === 'scheduled') && (
                    <button
                      className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input cursor-pointer border px-4 py-3 font-semibold disabled:opacity-50"
                      disabled={!reason.trim()}
                      onClick={() => void act('publish')}
                    >
                      Publish
                    </button>
                  )}
                  {event.state === 'published' && (
                    <button
                      className="border-admin-danger-border bg-admin-danger-bg text-admin-danger rounded-admin-input cursor-pointer border px-4 py-3 font-semibold disabled:opacity-50"
                      disabled={!reason.trim()}
                      onClick={() => void act('archive')}
                    >
                      Archive
                    </button>
                  )}
                </div>
              </div>
            ) : (
              <p className="text-admin-muted text-admin-note mt-5">
                Read-only access. Your role can inspect this event but cannot
                change its lifecycle.
              </p>
            )}
          </Panel>
          <Link
            to={`/events/${event.id}/edit`}
            className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input border px-4 py-3 text-center font-semibold no-underline"
          >
            Edit draft configuration
          </Link>
        </aside>
      </div>
    </section>
  )
}

function EventEditor() {
  const { token } = useAuth()
  const navigate = useNavigate()
  const { id } = useParams()
  const [event, setEvent] = useState<Partial<AdminEvent>>({
    slug: '',
    name: '',
    summary: '',
    description: '',
    starts_at: '',
    ends_at: '',
    reward_config: {},
  })
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  useEffect(() => {
    if (token && id)
      getEvent(token, id)
        .then(setEvent)
        .catch((cause) =>
          setError(
            cause instanceof Error ? cause.message : 'Failed to load event',
          ),
        )
  }, [id, token])
  const update = (key: keyof AdminEvent, value: string) =>
    setEvent({
      ...event,
      [key]: key.endsWith('_at') ? new Date(value).toISOString() : value,
    })
  const submit = async (form: React.FormEvent) => {
    form.preventDefault()
    if (!token) return
    try {
      const payload = {
        ...event,
        version: event.version ?? 0,
        reward_config: event.reward_config ?? {},
        reason,
      } as Parameters<typeof createEvent>[1]
      const saved = id
        ? await updateEvent(token, id, payload)
        : await createEvent(token, payload)
      navigate(`/events/${saved.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to save event')
    }
  }
  return (
    <form onSubmit={submit} className="mx-auto w-full max-w-360">
      <Link
        className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
        to="/events"
      >
        ← Back to events
      </Link>
      <PageHeader
        eyebrow="Live content"
        title={id ? 'Edit event' : 'Create event'}
      >
        <span className="text-admin-muted-subtle text-admin-caption self-end">
          {id ? `Version ${event.version ?? '...'}` : 'New draft'}
        </span>
      </PageHeader>
      {error && (
        <div
          role="alert"
          className="text-admin-danger border-admin-danger-border bg-admin-danger-bg my-4 rounded-lg border px-4 py-3"
        >
          {error}
        </div>
      )}
      <div className="gap-admin-17 mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,360px)] items-start max-[1050px]:grid-cols-1">
        <div className="grid gap-5">
          <Panel eyebrow="Catalog record" title="Player copy">
            <div className="grid grid-cols-[minmax(0,1fr)_180px] gap-4 max-[760px]:grid-cols-1">
              <Field label="Name">
                <input
                  className={inputClass}
                  value={event.name ?? ''}
                  onChange={(next) => update('name', next.target.value)}
                />
              </Field>
              <Field label="Slug">
                <input
                  className={inputClass}
                  value={event.slug ?? ''}
                  onChange={(next) => update('slug', next.target.value)}
                />
              </Field>
              <Field label="Summary" wide>
                <input
                  className={inputClass}
                  value={event.summary ?? ''}
                  onChange={(next) => update('summary', next.target.value)}
                />
              </Field>
              <Field label="Description" wide>
                <textarea
                  className={`${inputClass} min-h-32 resize-y`}
                  value={event.description ?? ''}
                  onChange={(next) => update('description', next.target.value)}
                />
              </Field>
            </div>
          </Panel>
          <Panel eyebrow="Versioned configuration" title="Reward payload">
            <Field label="JSON configuration">
              <textarea
                className={`${inputClass} min-h-36 resize-y font-mono text-sm`}
                value={JSON.stringify(event.reward_config, null, 2)}
                onChange={(next) => {
                  try {
                    setEvent({
                      ...event,
                      reward_config: JSON.parse(next.target.value),
                    })
                    setError('')
                  } catch {
                    setError('Reward configuration must be valid JSON')
                  }
                }}
              />
            </Field>
          </Panel>
        </div>
        <aside className="grid gap-5">
          <Panel eyebrow="Scheduling" title="Publication window">
            <div className="grid gap-4">
              <Field label="Starts at">
                <input
                  className={inputClass}
                  type="datetime-local"
                  value={event.starts_at ? event.starts_at.slice(0, 16) : ''}
                  onChange={(next) => update('starts_at', next.target.value)}
                />
              </Field>
              <Field label="Ends at">
                <input
                  className={inputClass}
                  type="datetime-local"
                  value={event.ends_at ? event.ends_at.slice(0, 16) : ''}
                  onChange={(next) => update('ends_at', next.target.value)}
                />
              </Field>
            </div>
          </Panel>
          <Panel eyebrow="Audit record" title="Save draft">
            <Field label="Change reason">
              <textarea
                className={`${inputClass} min-h-24 resize-y`}
                value={reason}
                placeholder="Why is this configuration changing?"
                onChange={(next) => setReason(next.target.value)}
              />
            </Field>
            <button
              className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input mt-4 w-full cursor-pointer border px-4 py-3 font-semibold disabled:cursor-not-allowed disabled:opacity-50"
              disabled={!reason.trim()}
              type="submit"
            >
              {id ? 'Save new revision' : 'Create draft'}
            </button>
          </Panel>
        </aside>
      </div>
    </form>
  )
}

function Field({
  label,
  children,
  wide = false,
}: {
  label: string
  children: ReactNode
  wide?: boolean
}) {
  return (
    <label
      className={`gap-admin-5 text-admin-field text-admin-muted grid min-w-0 ${wide ? 'col-span-full max-[760px]:col-auto' : ''}`}
    >
      <span className="text-admin-label font-mono tracking-wider uppercase">
        {label}
      </span>
      {children}
    </label>
  )
}
function formatRange(startsAt: string, endsAt: string) {
  return `${new Date(startsAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} – ${new Date(endsAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}`
}
