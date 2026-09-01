import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  createEvent,
  getEvent,
  getEvents,
  transitionEvent,
  updateEvent,
  type AdminEvent,
} from '../api/events'
import { useAuth } from '../hooks/useAuth'
const input =
  'rounded-admin-input border-admin-border-input bg-admin-canvas px-3 py-3 text-admin-ink-strong border outline-none w-full'
export function EventsPage() {
  const { token, admin } = useAuth()
  const [events, setEvents] = useState<AdminEvent[]>([])
  const [error, setError] = useState('')
  useEffect(() => {
    if (token)
      getEvents(token)
        .then((r) => setEvents(r.events))
        .catch((e) =>
          setError(e instanceof Error ? e.message : 'Failed to load events'),
        )
  }, [token])
  return (
    <section className="mx-auto max-w-360">
      <header className="border-admin-border border-b pb-8">
        <p className="text-admin-accent font-mono uppercase">Live content</p>
        <div className="flex items-end justify-between gap-4">
          <div>
            <h1 className="text-admin-ink-strong text-admin-hero">Events</h1>
            <p className="text-admin-muted">
              Schedule and publish versioned player experiences.
            </p>
          </div>
          {admin?.permissions.includes('events.manage') && (
            <Link
              to="/events/new"
              className="bg-admin-accent rounded-admin-input text-admin-button-ink px-4 py-3 font-bold no-underline"
            >
              Create event
            </Link>
          )}
        </div>
      </header>
      {error && <p role="alert">{error}</p>}
      <div className="mt-6 grid gap-4">
        {events.map((e) => (
          <Link
            key={e.id}
            to={`/events/${e.id}`}
            className="rounded-admin-panel border-admin-border bg-admin-surface-translucent grid gap-2 border p-5 text-inherit no-underline"
          >
            <div className="flex justify-between">
              <h2 className="m-0">{e.name}</h2>
              <span className="text-admin-accent font-mono uppercase">
                {e.state}
              </span>
            </div>
            <p className="text-admin-muted m-0">{e.summary}</p>
            <small>
              Revision {e.revision} · Resource version {e.version}
            </small>
          </Link>
        ))}
      </div>
    </section>
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
  const [error, setError] = useState('')
  useEffect(() => {
    if (token)
      getEvent(token, id)
        .then(setEvent)
        .catch((e) =>
          setError(e instanceof Error ? e.message : 'Failed to load event'),
        )
  }, [id, token])
  if (!event)
    return (
      <p role={error ? 'alert' : undefined}>{error || 'Loading event...'}</p>
    )
  const act = async (action: 'schedule' | 'publish' | 'archive') => {
    if (!token || !reason.trim()) return setError('A change reason is required')
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
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update event')
    }
  }
  return (
    <section className="mx-auto max-w-360">
      <Link to="/events" className="text-admin-accent">
        Back to events
      </Link>
      <div className="mt-5 grid gap-5 lg:grid-cols-[1fr_22rem]">
        <article
          className="rounded-admin-panel border-admin-border bg-admin-surface-translucent border p-7"
          style={{ borderTopColor: event.accent_color }}
        >
          <span className="text-admin-accent font-mono uppercase">
            Player preview · {event.state}
          </span>
          <h1 className="text-admin-ink-strong text-admin-hero">
            {event.name}
          </h1>
          <p className="text-admin-muted text-lg">{event.summary}</p>
          <p>{event.description}</p>
          <p>
            {new Date(event.starts_at).toLocaleString()} to{' '}
            {new Date(event.ends_at).toLocaleString()}
          </p>
        </article>
        <aside className="rounded-admin-panel border-admin-border border p-5">
          <h2>Lifecycle</h2>
          <p className="capitalize">{event.state}</p>
          <p>
            Revision {event.revision} · Version {event.version}
          </p>
          {admin?.permissions.includes('events.manage') && (
            <>
              <label className="grid gap-2">
                Change reason
                <textarea
                  aria-label="Change reason"
                  className={input}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
              </label>
              <div className="mt-4 grid gap-2">
                {event.state === 'draft' && (
                  <button onClick={() => act('schedule')}>Schedule</button>
                )}
                {(event.state === 'draft' || event.state === 'scheduled') && (
                  <button onClick={() => act('publish')}>Publish</button>
                )}
                {event.state === 'published' && (
                  <button onClick={() => act('archive')}>Archive</button>
                )}
              </div>
            </>
          )}
          {error && (
            <p role="alert" className="text-admin-danger">
              {error}
            </p>
          )}
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
    if (token && id) getEvent(token, id).then(setEvent)
  }, [id, token])
  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
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
    <form onSubmit={submit} className="mx-auto grid max-w-240 gap-4">
      <h1>{id ? 'Edit' : 'Create'} event</h1>
      {['slug', 'name', 'summary', 'description', 'starts_at', 'ends_at'].map(
        (key) => (
          <label key={key} className="grid gap-2 capitalize">
            {key.replace('_', ' ')}
            <input
              className={input}
              type={key.endsWith('_at') ? 'datetime-local' : 'text'}
              value={
                key.endsWith('_at') && event[key as keyof AdminEvent]
                  ? String(event[key as keyof AdminEvent]).slice(0, 16)
                  : String(event[key as keyof AdminEvent] ?? '')
              }
              onChange={(e) =>
                setEvent({
                  ...event,
                  [key]: key.endsWith('_at')
                    ? new Date(e.target.value).toISOString()
                    : e.target.value,
                })
              }
            />
          </label>
        ),
      )}
      <label className="grid gap-2">
        Reward configuration
        <textarea
          className={input}
          value={JSON.stringify(event.reward_config)}
          onChange={(e) => {
            try {
              setEvent({ ...event, reward_config: JSON.parse(e.target.value) })
            } catch {
              setError('Reward configuration must be valid JSON')
            }
          }}
        />
      </label>
      <label className="grid gap-2">
        Change reason
        <textarea
          className={input}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
      </label>
      {error && <p role="alert">{error}</p>}
      <button type="submit">Save draft</button>
    </form>
  )
}
