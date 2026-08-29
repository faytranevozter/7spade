import { useEffect, useEffectEvent, useState } from 'react'
import {
  getSessions,
  revokeOtherSessions,
  revokeSession,
  type AdminSession,
} from '../api/auth'
import { ApiError } from '../api/client'
import { getDashboard, type Dashboard } from '../api/dashboard'
import { EmptyState, SectionHeading } from '../components/InvestigationUI'
import { Notice, ReadOnlyNotice } from '../components/Feedback'
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function OverviewPage() {
  const { admin, token, error, refreshSession, expireSession } = useAuth()
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [dashboardError, setDashboardError] = useState('')
  const [loading, setLoading] = useState(false)
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [sessionMessage, setSessionMessage] = useState('')

  async function loadDashboard() {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
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
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 401) {
        expireSession(requestError.message)
        return
      }
      setDashboardError(
        requestError instanceof Error
          ? requestError.message
          : 'Failed to load dashboard',
      )
    } finally {
      setLoading(false)
    }
  }

  const refreshDashboard = useEffectEvent(loadDashboard)

  useEffect(() => {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
    const initial = window.setTimeout(() => void refreshDashboard(), 0)
    const interval = window.setInterval(() => void refreshDashboard(), 300000)
    return () => {
      window.clearTimeout(initial)
      window.clearInterval(interval)
    }
  }, [admin, token])
  useEffect(() => {
    if (!token) return
    getSessions(token)
      .then(setSessions)
      .catch((requestError: unknown) =>
        setSessionMessage(
          requestError instanceof Error
            ? requestError.message
            : 'Failed to load sessions',
        ),
      )
  }, [token])

  async function revoke(id: string, current: boolean) {
    if (
      !token ||
      !window.confirm(
        current
          ? 'Revoke this session and sign out?'
          : 'Revoke this administrator session?',
      )
    )
      return
    try {
      await revokeSession(token, id)
      if (current) expireSession('Current session revoked')
      else {
        setSessions((values) => values.filter((session) => session.id !== id))
        setSessionMessage('Session revoked')
      }
    } catch (requestError) {
      setSessionMessage(
        requestError instanceof Error
          ? requestError.message
          : 'Failed to revoke session',
      )
    }
  }
  async function revokeOthers() {
    if (!token || !window.confirm('Revoke all other administrator sessions?'))
      return
    try {
      await revokeOtherSessions(token)
      setSessions((values) => values.filter((session) => session.current))
      setSessionMessage('All other sessions revoked')
    } catch (requestError) {
      setSessionMessage(
        requestError instanceof Error
          ? requestError.message
          : 'Failed to revoke sessions',
      )
    }
  }
  if (!admin) return null

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="overview-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            System status / {new Date().toISOString().slice(0, 10)}
          </p>
          <h1
            id="overview-heading"
            className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            Operations overview
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            A current pulse of platform activity, dependency health, and your
            administrator security posture.
          </p>
        </div>
        {admin.permissions.includes('dashboard.read') ? (
          <div className="flex items-center gap-[0.6rem] max-[760px]:flex-col max-[760px]:items-stretch">
            <span className="border-admin-accent/40 bg-admin-accent/8 text-admin-accent-bright rounded-[7px] border px-[0.7rem] py-[0.55rem] font-mono text-[0.6rem]">
              {dashboard?.environment?.toUpperCase() ?? 'LOADING'}
            </span>
            <button
              onClick={() => void loadDashboard()}
              disabled={loading}
              className="border-admin-accent/40 cursor-pointer rounded-[7px] border bg-transparent px-[0.7rem] py-[0.55rem] font-mono text-[0.6rem] text-[#d9d4c8] disabled:opacity-45"
            >
              {loading ? 'Refreshing...' : 'Refresh snapshot'}
            </button>
          </div>
        ) : null}
      </header>
      {error ? <Notice variant="error">{error}</Notice> : null}
      {dashboardError ? (
        <Notice variant="error">{dashboardError}</Notice>
      ) : null}
      {!admin.permissions.includes('dashboard.read') ? (
        <ReadOnlyNotice title="Dashboard access limited">
          Your administrator account does not have dashboard access.
        </ReadOnlyNotice>
      ) : null}
      {dashboard ? (
        <section
          className="mt-6 grid grid-cols-5 gap-3 max-[1100px]:grid-cols-3 max-[760px]:grid-cols-2 max-[480px]:grid-cols-1"
          aria-label="Current activity"
        >
          <PulseMetric
            label="Active players"
            value={dashboard.current?.players ?? 0}
            detail="Connected now"
          />
          <PulseMetric
            label="Active rooms"
            value={dashboard.current?.rooms ?? 0}
            detail="Waiting or playing"
          />
          <PulseMetric
            label="Games in progress"
            value={dashboard.current?.games ?? 0}
            detail="Authoritative WS state"
          />
          <HealthPulse
            label="API"
            status={dashboard.services?.api.status ?? dashboard.status}
          />
          <HealthPulse
            label="WebSocket"
            status={dashboard.services?.ws.status ?? 'not_configured'}
          />
        </section>
      ) : null}
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(300px,370px)] items-start gap-6 max-[1100px]:grid-cols-1">
        {dashboard ? (
          <main>
            <ActivityWindow
              title="Today"
              window={dashboard.windows?.day}
              data={dashboard.daily}
            />
            <ActivityWindow
              title="This month"
              window={dashboard.windows?.month}
              data={dashboard.monthly}
            />
            <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5">
              <SectionHeading
                eyebrow="Authoritative systems"
                title="Operational tools"
                id="tools-heading"
                meta={dashboard.links?.length ?? 0}
              />
              {dashboard.links?.length ? (
                <div className="mt-4 grid grid-cols-2 gap-[0.6rem] max-[760px]:grid-cols-1">
                  {dashboard.links.map((link) => (
                    <a
                      key={link.name}
                      href={link.url}
                      target="_blank"
                      rel="noreferrer"
                      className="border-admin-ink/10 hover:border-admin-accent/40 rounded-lg border p-[0.8rem] no-underline"
                    >
                      <span className="text-admin-ink block text-[0.72rem]">
                        {formatLabel(link.name)}
                      </span>
                      <small className="text-admin-muted-subtle mt-1 block text-[0.58rem]">
                        Open external system
                      </small>
                    </a>
                  ))}
                </div>
              ) : (
                <EmptyState
                  mark="O"
                  title="No tools configured"
                  description="Operational links have not been configured for this environment."
                />
              )}
            </section>
          </main>
        ) : (
          <div />
        )}
        <aside className="sticky top-6 max-[1100px]:static">
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5">
            <div className="flex items-start justify-between gap-[0.7rem] [&_.section-heading]:flex-1">
              <SectionHeading
                eyebrow="Your security"
                title="Active sessions"
                id="sessions-heading"
                meta={sessions.length}
              />
              {sessions.some((session) => !session.current) ? (
                <button
                  onClick={() => void revokeOthers()}
                  className="text-admin-danger cursor-pointer rounded-[7px] border border-[#c0392b]/55 bg-transparent px-[0.7rem] py-[0.55rem] font-mono text-[0.6rem]"
                >
                  Revoke all others
                </button>
              ) : null}
            </div>
            {sessionMessage ? (
              <Notice variant="success">{sessionMessage}</Notice>
            ) : null}
            <div className="mt-4 grid gap-[0.55rem]">
              {sessions.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  onRevoke={revoke}
                />
              ))}
            </div>
          </section>
        </aside>
      </div>
    </section>
  )
}

function PulseMetric({
  label,
  value,
  detail,
}: {
  label: string
  value: number
  detail: string
}) {
  return (
    <article className="border-admin-ink/10 bg-admin-surface/88 rounded-[11px] border p-4">
      <span className="text-admin-muted-subtle block font-mono text-[0.57rem] uppercase">
        {label}
      </span>
      <strong className="text-admin-ink-strong my-2 block text-[1.55rem] font-medium">
        {value}
      </strong>
      <small className="text-admin-muted-subtle block text-[0.58rem]">
        {detail}
      </small>
    </article>
  )
}
function HealthPulse({ label, status }: { label: string; status: string }) {
  const healthy = status === 'ok'
  return (
    <article
      className={`border-admin-ink/10 bg-admin-surface/88 rounded-[11px] border p-4 ${healthy ? 'border-t-[#2d7a46]' : 'border-t-[#c0392b]'}`}
    >
      <span className="text-admin-muted-subtle block font-mono text-[0.57rem] uppercase">
        {label}
      </span>
      <strong
        className={`my-2 block text-[0.9rem] font-medium ${healthy ? 'text-admin-success' : 'text-admin-danger'}`}
      >
        {formatLabel(status)}
      </strong>
      <small className="text-admin-muted-subtle block text-[0.58rem]">
        {healthy ? 'Service responding' : 'Needs attention'}
      </small>
    </article>
  )
}
function ActivityWindow({
  title,
  window,
  data,
}: {
  title: string
  window?: { from: string; to: string }
  data?: Dashboard['daily']
}) {
  const metrics: Array<[string, number | string]> = [
    ['Registrations', data?.registrations ?? 0],
    ['Players active', data?.players ?? 0],
    ['Rooms created', data?.rooms ?? 0],
    ['Games started', data?.games_started ?? 0],
    ['Completed', data?.games_completed ?? 0],
    ['Abandoned', data?.games_abandoned ?? 0],
    ['Avg. duration', formatDuration(data?.average_game_duration_seconds ?? 0)],
  ]
  return (
    <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5">
      <SectionHeading
        eyebrow="UTC activity window"
        title={title}
        id={`activity-${title}`}
        meta={
          window
            ? `${new Date(window.from).toLocaleDateString()} - ${new Date(window.to).toLocaleDateString()}`
            : undefined
        }
      />
      <div className="mt-4 grid grid-cols-4 gap-[0.6rem] max-[1100px]:grid-cols-3 max-[760px]:grid-cols-2 max-[480px]:grid-cols-1">
        {metrics.map(([label, value]) => (
          <div
            key={label}
            className="border-admin-ink/8 rounded-[7px] border p-3"
          >
            <span className="text-admin-muted-subtle block text-[0.6rem]">
              {label}
            </span>
            <strong className="text-admin-ink mt-[0.4rem] block font-mono text-[1rem]">
              {value}
            </strong>
          </div>
        ))}
      </div>
    </section>
  )
}
function SessionRow({
  session,
  onRevoke,
}: {
  session: AdminSession
  onRevoke: (id: string, current: boolean) => Promise<void>
}) {
  return (
    <article
      className={`border-admin-ink/8 grid grid-cols-[38px_minmax(0,1fr)] gap-[0.65rem] rounded-lg border p-[0.7rem] ${session.current ? 'border-[#2d7a46]/30' : ''}`}
    >
      <span className="text-admin-muted grid size-8.5 place-items-center rounded-[7px] bg-white/4 text-center text-[0.52rem]">
        {session.current ? 'This device' : 'Device'}
      </span>
      <div>
        <strong className="text-[0.7rem] text-[#d9d4c8]">
          {session.current
            ? 'Current session'
            : session.user_agent || 'Unknown device'}
        </strong>
        <p className="text-admin-muted my-[0.2rem] text-[0.62rem]">
          {session.ip_address || 'Unknown IP'}
        </p>
        <small className="text-[0.56rem] text-[#60645e]">
          Started {formatDateTime(session.created_at)}
        </small>
      </div>
      <button
        className="text-admin-danger col-start-2 w-max cursor-pointer border-0 bg-transparent p-0 text-[0.59rem]"
        onClick={() => void onRevoke(session.id, session.current)}
      >
        {session.current ? 'Revoke and sign out' : 'Revoke'}
      </button>
    </article>
  )
}
function formatDuration(seconds: number) {
  return seconds < 60
    ? `${seconds}s`
    : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}
