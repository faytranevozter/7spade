import { useEffect, useEffectEvent, useState } from 'react'
import { Link } from 'react-router'
import {
  getSessions,
  revokeOtherSessions,
  revokeSession,
  type AdminSession,
} from '../api/auth'
import { getAuditEvents, type AuditEvent } from '../api/audit'
import { ApiError } from '../api/client'
import {
  getDashboard,
  type ActivitySummary,
  type Dashboard,
} from '../api/dashboard'
import { getEvents, type AdminEvent } from '../api/events'
import { Notice, ReadOnlyNotice } from '../components/Feedback'
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

type LoadState = 'loading' | 'ready' | 'error'

export function OverviewPage() {
  const { admin, token, error, refreshSession, expireSession } = useAuth()
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [dashboardError, setDashboardError] = useState('')
  const [loading, setLoading] = useState(false)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [period, setPeriod] = useState<'today' | 'month'>('today')
  const [events, setEvents] = useState<AdminEvent[]>([])
  const [eventsState, setEventsState] = useState<LoadState>('loading')
  const [auditEvents, setAuditEvents] = useState<AuditEvent[]>([])
  const [auditState, setAuditState] = useState<LoadState>('loading')
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [sessionsState, setSessionsState] = useState<LoadState>('loading')
  const [sessionMessage, setSessionMessage] = useState('')
  const [sessionError, setSessionError] = useState('')
  const [revoking, setRevoking] = useState<string | null>(null)
  const canReadDashboard =
    admin?.permissions.includes('dashboard.read') ?? false
  const canReadEvents = admin?.permissions.includes('events.read') ?? false
  const canReadAudit = admin?.permissions.includes('audit.read') ?? false

  async function loadDashboard() {
    if (!token || !canReadDashboard) return
    setLoading(true)
    setDashboardError('')
    try {
      const result = await getDashboard(token).catch(
        async (requestError: unknown) => {
          if (
            !(requestError instanceof ApiError) ||
            requestError.status !== 401
          )
            throw requestError
          return getDashboard(await refreshSession())
        },
      )
      setDashboard(result)
      setLastUpdated(new Date())
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 401) {
        expireSession(requestError.message)
        return
      }
      setDashboardError(errorMessage(requestError, 'Failed to load dashboard'))
    } finally {
      setLoading(false)
    }
  }

  const refreshDashboard = useEffectEvent(loadDashboard)

  useEffect(() => {
    if (!token || !canReadDashboard) return
    const initial = window.setTimeout(() => void refreshDashboard(), 0)
    const interval = window.setInterval(() => void refreshDashboard(), 300000)
    return () => {
      window.clearTimeout(initial)
      window.clearInterval(interval)
    }
  }, [canReadDashboard, token])

  useEffect(() => {
    if (!token) return
    getSessions(token)
      .then((result) => {
        setSessions(result)
        setSessionError('')
        setSessionsState('ready')
      })
      .catch((requestError: unknown) => {
        setSessionError(errorMessage(requestError, 'Failed to load sessions'))
        setSessionsState('error')
      })
  }, [token])

  useEffect(() => {
    if (!token || !canReadEvents) return
    getEvents(token)
      .then(({ events: result }) => {
        setEvents(result ?? [])
        setEventsState('ready')
      })
      .catch(() => setEventsState('error'))
  }, [canReadEvents, token])

  useEffect(() => {
    if (!token || !canReadAudit) return
    getAuditEvents(token, {}, 6, 0)
      .then(({ events: result }) => {
        setAuditEvents(result ?? [])
        setAuditState('ready')
      })
      .catch(() => setAuditState('error'))
  }, [canReadAudit, token])

  async function revoke(id: string, current: boolean) {
    if (
      !token ||
      revoking ||
      !window.confirm(
        current
          ? 'Revoke this session and sign out?'
          : 'Revoke this administrator session?',
      )
    )
      return
    setRevoking(id)
    setSessionError('')
    setSessionMessage('')
    try {
      await revokeSession(token, id)
      if (current) expireSession('Current session revoked')
      else {
        setSessions((values) => values.filter((session) => session.id !== id))
        setSessionMessage('Session revoked')
      }
    } catch (requestError) {
      setSessionError(errorMessage(requestError, 'Failed to revoke session'))
    } finally {
      setRevoking(null)
    }
  }

  async function revokeOthers() {
    if (
      !token ||
      revoking ||
      !window.confirm('Revoke all other administrator sessions?')
    )
      return
    setRevoking('others')
    setSessionError('')
    setSessionMessage('')
    try {
      await revokeOtherSessions(token)
      setSessions((values) => values.filter((session) => session.current))
      setSessionMessage('All other sessions revoked')
    } catch (requestError) {
      setSessionError(errorMessage(requestError, 'Failed to revoke sessions'))
    } finally {
      setRevoking(null)
    }
  }

  if (!admin) return null

  const serviceIssues = dashboard ? getServiceIssues(dashboard) : []
  const attention = [
    ...serviceIssues,
    ...(!admin.mfa_enrolled
      ? [
          {
            title: 'MFA is not enrolled',
            detail: 'Protect this administrator identity with a second factor.',
          },
        ]
      : []),
  ]
  const summary = period === 'today' ? dashboard?.daily : dashboard?.monthly
  const reportWindow =
    period === 'today' ? dashboard?.windows?.day : dashboard?.windows?.month
  const relevantEvents = [...events]
    .filter(
      (event) => event.state === 'published' || event.state === 'scheduled',
    )
    .sort(
      (a, b) =>
        new Date(a.starts_at).getTime() - new Date(b.starts_at).getTime(),
    )
    .slice(0, 4)

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="overview-heading"
    >
      <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-6 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Operations command center
          </p>
          <h1
            id="overview-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-7 text-admin-detail-hero leading-none font-medium tracking-tighter"
          >
            Platform overview
          </h1>
          <p className="text-admin-muted text-admin-body m-0 max-w-180 leading-relaxed">
            Monitor live activity, service health, scheduled content, and
            administrator security from one workspace.
          </p>
        </div>
        {canReadDashboard ? (
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge
              status={
                dashboard?.status ?? (loading ? 'loading' : 'unavailable')
              }
            />
            <span className="border-admin-border-input text-admin-muted rounded-admin-input text-admin-caption border px-3 py-2 font-mono">
              {dashboard?.environment?.toUpperCase() ?? 'ENVIRONMENT UNKNOWN'}
            </span>
            <button
              type="button"
              onClick={() => void loadDashboard()}
              disabled={loading}
              className="border-admin-accent-border text-admin-accent-bright rounded-admin-input text-admin-field cursor-pointer border bg-transparent px-3 py-2 font-semibold disabled:cursor-wait disabled:opacity-50"
            >
              {loading ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
        ) : null}
      </header>

      {error ? <Notice variant="error">{error}</Notice> : null}
      {dashboardError ? (
        <Notice variant="error">
          Snapshot refresh failed: {dashboardError}.{' '}
          {dashboard ? 'Showing the last successful snapshot.' : ''}
        </Notice>
      ) : null}
      {!canReadDashboard ? (
        <ReadOnlyNotice title="Dashboard access limited">
          Your administrator account does not have dashboard access.
        </ReadOnlyNotice>
      ) : null}

      {dashboard ? (
        <>
          <section
            className={`rounded-admin-panel mt-6 border p-5 ${attention.length ? 'border-admin-warning/35 bg-admin-warning/5' : 'border-admin-success-border bg-admin-success-bg'}`}
            aria-labelledby="attention-heading"
          >
            <div className="flex items-center justify-between gap-4">
              <div>
                <p
                  className={`text-admin-label m-0 font-mono tracking-wider uppercase ${attention.length ? 'text-admin-warning' : 'text-admin-success'}`}
                >
                  {attention.length ? 'Review required' : 'All clear'}
                </p>
                <h2
                  id="attention-heading"
                  className="text-admin-ink-strong text-admin-section mt-1 mb-0"
                >
                {attention.length
                  ? `${attention.length} item${attention.length === 1 ? ' needs' : 's need'} attention`
                    : 'No detected operational issues'}
                </h2>
              </div>
              <span
                className={`text-admin-heading grid size-10 shrink-0 place-items-center rounded-full border ${attention.length ? 'border-admin-warning/35 text-admin-warning' : 'border-admin-success-border text-admin-success'}`}
                aria-hidden="true"
              >
                {attention.length ? '!' : '✓'}
              </span>
            </div>
            {attention.length ? (
              <div className="mt-4 grid gap-2 sm:grid-cols-2">
                {attention.map((item) => (
                  <article
                    key={item.title}
                    className="border-admin-border-faint bg-admin-canvas/40 rounded-admin-input border p-3"
                  >
                    <strong className="text-admin-ink text-admin-field block">
                      {item.title}
                    </strong>
                    <p className="text-admin-muted text-admin-caption mt-1 mb-0 leading-relaxed">
                      {item.detail}
                    </p>
                  </article>
                ))}
              </div>
            ) : (
              <p className="text-admin-muted text-admin-caption mt-2 mb-0">
                API and WebSocket health checks are responding normally.
              </p>
            )}
          </section>

          <section
            className="mt-4 grid grid-cols-3 gap-3 max-[640px]:grid-cols-1"
            aria-label="Current activity"
          >
            <LiveMetric
              label="Connected players"
              value={dashboard.current?.players}
              detail="Present now"
              to={
                admin.permissions.includes('users.read') ? '/users' : undefined
              }
            />
            <LiveMetric
              label="Active rooms"
              value={dashboard.current?.rooms}
              detail="Waiting or playing"
              to={
                admin.permissions.includes('rooms.read') ? '/rooms' : undefined
              }
            />
            <LiveMetric
              label="Games in progress"
              value={dashboard.current?.games}
              detail="Authoritative WS state"
              to={
                admin.permissions.includes('games.read') ? '/games' : undefined
              }
            />
          </section>
          <p className="text-admin-muted-subtle text-admin-caption mt-2 mb-0 text-right font-mono">
            Snapshot{' '}
            {lastUpdated
              ? `updated ${lastUpdated.toLocaleTimeString()}`
              : 'time unavailable'}{' '}
            · auto-refreshes every 5 minutes
          </p>
        </>
      ) : loading ? (
        <LoadingPanel label="Loading operational snapshot" />
      ) : null}

      <div className="mt-6 grid grid-cols-[minmax(0,1.55fr)_minmax(300px,0.85fr)] items-start gap-6 max-[1050px]:grid-cols-1">
        <div className="grid min-w-0 gap-6">
          {dashboard ? (
            <ActivityPanel
              period={period}
              onPeriodChange={setPeriod}
              data={summary}
              window={reportWindow}
            />
          ) : null}
          {canReadEvents ? (
            <EventsPanel state={eventsState} events={relevantEvents} />
          ) : null}
          {dashboard ? (
            <ToolsPanel dashboard={dashboard} permissions={admin.permissions} />
          ) : null}
        </div>
        <aside className="grid min-w-0 gap-6 xl:sticky xl:top-6">
          {canReadAudit ? (
            <AuditPanel state={auditState} events={auditEvents} />
          ) : null}
          <SecurityPanel
            adminMfa={admin.mfa_enrolled ?? false}
            state={sessionsState}
            sessions={sessions}
            message={sessionMessage}
            error={sessionError}
            revoking={revoking}
            onRevoke={revoke}
            onRevokeOthers={revokeOthers}
          />
        </aside>
      </div>
    </section>
  )
}

const panelClass =
  'border-admin-border-subtle bg-admin-surface-translucent shadow-admin-panel rounded-admin-panel border p-5'

function LiveMetric({
  label,
  value,
  detail,
  to,
}: {
  label: string
  value?: number
  detail: string
  to?: string
}) {
  const content = (
    <>
      <span className="text-admin-muted-subtle text-admin-label block font-mono uppercase">
        {label}
      </span>
      <strong className="text-admin-ink-strong my-2 block text-4xl font-medium">
        {value ?? '—'}
      </strong>
      <span className="text-admin-muted text-admin-caption">
        {detail}
        {to ? ' · View details →' : ''}
      </span>
    </>
  )
  return to ? (
    <Link
      to={to}
      className="border-admin-border-faint bg-admin-surface/88 hover:border-admin-accent-border rounded-admin-rule border p-4 text-inherit no-underline transition-colors"
    >
      {content}
    </Link>
  ) : (
    <article className="border-admin-border-faint bg-admin-surface/88 rounded-admin-rule border p-4">
      {content}
    </article>
  )
}

function ActivityPanel({
  period,
  onPeriodChange,
  data,
  window,
}: {
  period: 'today' | 'month'
  onPeriodChange: (period: 'today' | 'month') => void
  data?: ActivitySummary
  window?: { from: string; to: string }
}) {
  const metrics: Array<[string, number | string | undefined]> = [
    ['Registrations', data?.registrations],
    ['Active players', data?.players],
    ['Rooms created', data?.rooms],
    ['Games started', data?.games_started],
    ['Completed', data?.games_completed],
    ['Abandoned', data?.games_abandoned],
    [
      'Average duration',
      data ? formatDuration(data.average_game_duration_seconds) : undefined,
    ],
  ]
  return (
    <section className={panelClass} aria-labelledby="activity-heading">
      <div className="flex items-start justify-between gap-4 max-[520px]:flex-col">
        <div>
          <p className="text-admin-accent text-admin-label m-0 font-mono tracking-wider uppercase">
            UTC reporting window
          </p>
          <h2
            id="activity-heading"
            className="text-admin-ink-strong text-admin-section mt-1 mb-0"
          >
            Activity summary
          </h2>
          <p className="text-admin-muted text-admin-caption mt-2 mb-0">
            {window
              ? `${formatDate(window.from)} through ${formatDate(window.to)}`
              : 'Reporting period unavailable'}
          </p>
        </div>
        <div
          className="border-admin-border-input bg-admin-canvas rounded-admin-input grid grid-cols-2 border p-1"
          aria-label="Activity period"
        >
          {(['today', 'month'] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={period === value}
              onClick={() => onPeriodChange(value)}
              className={`text-admin-caption rounded-md px-3 py-2 font-semibold ${period === value ? 'bg-admin-accent-soft text-admin-accent-bright' : 'text-admin-muted bg-transparent'}`}
            >
              {value === 'today' ? 'Today' : 'This month'}
            </button>
          ))}
        </div>
      </div>
      <div className="mt-5 grid grid-cols-4 gap-2 max-[720px]:grid-cols-2 max-[380px]:grid-cols-1">
        {metrics.map(([label, value]) => (
          <div
            key={label}
            className="border-admin-border-faint rounded-admin-input border p-3"
          >
            <span className="text-admin-muted-subtle text-admin-caption block">
              {label}
            </span>
            <strong className="text-admin-ink text-admin-preview mt-1 block font-mono">
              {value ?? '—'}
            </strong>
          </div>
        ))}
      </div>
    </section>
  )
}

function EventsPanel({
  state,
  events,
}: {
  state: LoadState
  events: AdminEvent[]
}) {
  return (
    <section className={panelClass} aria-labelledby="events-overview-heading">
      <SectionTitle
        eyebrow="Content schedule"
        title="Current and upcoming events"
        id="events-overview-heading"
        action={
          <Link to="/events" className="text-admin-accent text-admin-caption">
            View all events →
          </Link>
        }
      />
      {state === 'loading' ? (
        <LoadingLine />
      ) : state === 'error' ? (
        <InlineState tone="error" text="Event schedule could not be loaded." />
      ) : events.length ? (
        <div className="mt-4 grid gap-2">
          {events.map((event) => (
            <Link
              key={event.id}
              to={`/events/${event.id}`}
              className="border-admin-border-faint hover:border-admin-accent-border rounded-admin-input flex items-center justify-between gap-4 border p-3 text-inherit no-underline max-[520px]:items-start"
            >
              <div className="min-w-0">
                <strong className="text-admin-ink text-admin-field block">
                  {event.name}
                </strong>
                <span className="text-admin-muted text-admin-caption mt-1 block">
                  {event.state === 'published' &&
                  new Date(event.starts_at) <= new Date() &&
                  new Date(event.ends_at) >= new Date()
                    ? 'Active now'
                    : `Starts ${formatDateTime(event.starts_at)}`}
                </span>
              </div>
              <span className="border-admin-border-input text-admin-muted text-admin-label shrink-0 rounded-full border px-2 py-1 font-mono uppercase">
                {event.state}
              </span>
            </Link>
          ))}
        </div>
      ) : (
        <InlineState text="No published or scheduled events." />
      )}
    </section>
  )
}

function AuditPanel({
  state,
  events,
}: {
  state: LoadState
  events: AuditEvent[]
}) {
  return (
    <section className={panelClass} aria-labelledby="recent-audit-heading">
      <SectionTitle
        eyebrow="Recent changes"
        title="Admin activity"
        id="recent-audit-heading"
        action={
          <Link
            to="/audit-events"
            className="text-admin-accent text-admin-caption"
          >
            Full log →
          </Link>
        }
      />
      {state === 'loading' ? (
        <LoadingLine />
      ) : state === 'error' ? (
        <InlineState
          tone="error"
          text="Recent audit activity could not be loaded."
        />
      ) : events.length ? (
        <div className="mt-4 grid gap-0">
          {events.map((event) => (
            <article
              key={event.id}
              className="border-admin-border-divider grid grid-cols-[10px_minmax(0,1fr)] gap-3 border-b py-3 last:border-0"
            >
              <span
                className={`mt-1.5 size-2 rounded-full ${event.outcome === 'success' ? 'bg-admin-success' : 'bg-admin-danger'}`}
              />
              <div className="min-w-0">
                <strong className="text-admin-ink-soft text-admin-caption block wrap-anywhere">
                  {formatLabel(event.action)}
                </strong>
                <span className="text-admin-muted-subtle text-admin-label mt-1 block">
                  {formatDateTime(event.occurred_at)}
                  {event.resource_type
                    ? ` · ${formatLabel(event.resource_type)}`
                    : ''}
                </span>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <InlineState text="No recent audit activity." />
      )}
    </section>
  )
}

function SecurityPanel({
  adminMfa,
  state,
  sessions,
  message,
  error,
  revoking,
  onRevoke,
  onRevokeOthers,
}: {
  adminMfa: boolean
  state: LoadState
  sessions: AdminSession[]
  message: string
  error: string
  revoking: string | null
  onRevoke: (id: string, current: boolean) => Promise<void>
  onRevokeOthers: () => Promise<void>
}) {
  const others = sessions.filter((session) => !session.current)
  return (
    <section className={panelClass} aria-labelledby="security-heading">
      <SectionTitle
        eyebrow="Your security"
        title="Administrator access"
        id="security-heading"
      />
      <div className="border-admin-border-faint rounded-admin-input mt-4 grid grid-cols-2 border">
        <div className="p-3">
          <span className="text-admin-muted-subtle text-admin-label block uppercase">
            MFA
          </span>
          <strong
            className={`text-admin-field mt-1 block ${adminMfa ? 'text-admin-success' : 'text-admin-warning'}`}
          >
            {adminMfa ? 'Enrolled' : 'Not enrolled'}
          </strong>
        </div>
        <div className="border-admin-border-divider border-l p-3">
          <span className="text-admin-muted-subtle text-admin-label block uppercase">
            Sessions
          </span>
          <strong className="text-admin-ink text-admin-field mt-1 block">
            {state === 'ready' ? sessions.length : '—'}
          </strong>
        </div>
      </div>
      {message ? <Notice variant="success">{message}</Notice> : null}
      {error ? <Notice variant="error">{error}</Notice> : null}
      {state === 'loading' ? (
        <LoadingLine />
      ) : state === 'error' ? null : (
        <details className="border-admin-border-faint rounded-admin-input mt-4 border">
          <summary className="text-admin-ink text-admin-field cursor-pointer p-3 font-semibold">
            Review active sessions
          </summary>
          <div className="border-admin-border-divider grid gap-2 border-t p-3">
            {sessions.map((session) => (
              <SessionRow
                key={session.id}
                session={session}
                pending={revoking === session.id}
                disabled={revoking !== null}
                onRevoke={onRevoke}
              />
            ))}
            {others.length ? (
              <button
                type="button"
                disabled={revoking !== null}
                onClick={() => void onRevokeOthers()}
                className="border-admin-danger-border text-admin-danger rounded-admin-input text-admin-caption mt-1 cursor-pointer border bg-transparent px-3 py-2 disabled:cursor-wait disabled:opacity-50"
              >
                {revoking === 'others'
                  ? 'Revoking...'
                  : 'Revoke all other sessions'}
              </button>
            ) : null}
          </div>
        </details>
      )}
    </section>
  )
}

function ToolsPanel({
  dashboard,
  permissions,
}: {
  dashboard: Dashboard
  permissions: string[]
}) {
  const shortcuts = [
    { permission: 'users.read', to: '/users', label: 'Investigate users' },
    { permission: 'rooms.read', to: '/rooms', label: 'Inspect rooms' },
    { permission: 'games.read', to: '/games', label: 'Review games' },
    { permission: 'events.read', to: '/events', label: 'Manage events' },
    {
      permission: 'settings.read',
      to: '/settings',
      label: 'Application settings',
    },
  ].filter((item) => permissions.includes(item.permission))
  return (
    <section className={panelClass} aria-labelledby="tools-heading">
      <SectionTitle
        eyebrow="Fast paths"
        title="Operational shortcuts"
        id="tools-heading"
      />
      <div className="mt-4 grid grid-cols-2 gap-2 max-[520px]:grid-cols-1">
        {shortcuts.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className="border-admin-border-faint hover:border-admin-accent-border rounded-admin-input text-admin-field text-admin-ink border p-3 no-underline"
          >
            {item.label} <span className="text-admin-accent">→</span>
          </Link>
        ))}
        {dashboard.links?.map((link) => (
          <a
            key={link.name}
            href={link.url}
            target="_blank"
            rel="noreferrer"
            className="border-admin-border-faint hover:border-admin-accent-border rounded-admin-input text-admin-field text-admin-ink border p-3 no-underline"
          >
            {formatLabel(link.name)}{' '}
            <span className="text-admin-accent">↗</span>
          </a>
        ))}
      </div>
      {!shortcuts.length && !dashboard.links?.length ? (
        <InlineState text="No operational tools are available for this account." />
      ) : null}
    </section>
  )
}

function SessionRow({
  session,
  pending,
  disabled,
  onRevoke,
}: {
  session: AdminSession
  pending: boolean
  disabled: boolean
  onRevoke: (id: string, current: boolean) => Promise<void>
}) {
  return (
    <article
      className={`border-admin-border-faint rounded-admin-input border p-3 ${session.current ? 'border-admin-success-border' : ''}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <strong className="text-admin-ink-soft text-admin-caption block">
            {session.current
              ? 'Current session'
              : session.user_agent || 'Unknown device'}
          </strong>
          <span className="text-admin-muted-subtle text-admin-label mt-1 block wrap-anywhere">
            {session.ip_address || 'Unknown IP'} ·{' '}
            {formatDateTime(session.created_at)}
          </span>
        </div>
        <button
          type="button"
          disabled={disabled}
          onClick={() => void onRevoke(session.id, session.current)}
          className="text-admin-danger text-admin-label shrink-0 cursor-pointer border-0 bg-transparent p-0 disabled:cursor-wait disabled:opacity-50"
        >
          {pending ? 'Revoking...' : session.current ? 'Sign out' : 'Revoke'}
        </button>
      </div>
    </article>
  )
}

function SectionTitle({
  eyebrow,
  title,
  id,
  action,
}: {
  eyebrow: string
  title: string
  id: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <p className="text-admin-accent text-admin-label m-0 font-mono tracking-wider uppercase">
          {eyebrow}
        </p>
        <h2
          id={id}
          className="text-admin-ink-strong text-admin-section mt-1 mb-0"
        >
          {title}
        </h2>
      </div>
      {action}
    </div>
  )
}
function StatusBadge({ status }: { status: string }) {
  const healthy = status === 'ready' || status === 'ok'
  return (
    <span
      className={`rounded-admin-input text-admin-caption border px-3 py-2 font-mono uppercase ${healthy ? 'border-admin-success-border bg-admin-success-bg text-admin-success' : 'border-admin-danger-border bg-admin-danger-bg text-admin-danger'}`}
    >
      {formatLabel(status)}
    </span>
  )
}
function LoadingPanel({ label }: { label: string }) {
  return (
    <div
      className={`${panelClass} text-admin-muted text-admin-field mt-6 animate-pulse`}
    >
      {label}...
    </div>
  )
}
function LoadingLine() {
  return (
    <div
      className="bg-admin-border-faint rounded-admin-input mt-4 h-14 animate-pulse"
      aria-label="Loading"
    />
  )
}
function InlineState({
  text,
  tone = 'muted',
}: {
  text: string
  tone?: 'muted' | 'error'
}) {
  return (
    <p
      className={`rounded-admin-input text-admin-caption mt-4 mb-0 border p-3 ${tone === 'error' ? 'border-admin-danger-border bg-admin-danger-bg text-admin-danger' : 'border-admin-border-faint text-admin-muted'}`}
    >
      {text}
    </p>
  )
}
function getServiceIssues(dashboard: Dashboard) {
  return (
    [
      ['API', dashboard.services?.api.status],
      ['WebSocket', dashboard.services?.ws.status],
    ] as const
  )
    .filter(([, status]) => status !== 'ok')
    .map(([name, status]) => ({
      title: `${name} ${formatLabel(status || 'unavailable')}`,
      detail:
        status === 'not_configured'
          ? 'A health endpoint is not configured for this environment.'
          : 'The service health check needs investigation.',
    }))
}
function errorMessage(value: unknown, fallback: string) {
  return value instanceof Error ? value.message : fallback
}
function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds)) return '—'
  return seconds < 60
    ? `${seconds}s`
    : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}
function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, {
    dateStyle: 'medium',
    timeZone: 'UTC',
  })
}
