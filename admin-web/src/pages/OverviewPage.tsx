import { useEffect, useState } from 'react'
import { getSessions, revokeOtherSessions, revokeSession, type AdminSession } from '../api/auth'
import { ApiError } from '../api/client'
import { getDashboard, type Dashboard } from '../api/dashboard'
import { useAuth } from '../hooks/useAuth'

export function OverviewPage() {
  const { admin, token, error, refreshSession, expireSession } = useAuth()
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [dashboardError, setDashboardError] = useState('')
  const [dashboardLoading, setDashboardLoading] = useState(false)
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [sessionMessage, setSessionMessage] = useState('')

  async function loadDashboard() {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
    setDashboardLoading(true)
    setDashboardError('')
    try {
      const result = await getDashboard(token).catch(async (requestError: unknown) => {
        if (!(requestError instanceof ApiError) || requestError.status !== 401) throw requestError
        return getDashboard(await refreshSession())
      })
      setDashboard(result)
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 401) {
        expireSession(requestError.message)
        return
      }
      setDashboardError(requestError instanceof Error ? requestError.message : 'Failed to load dashboard')
    } finally {
      setDashboardLoading(false)
    }
  }

  // The dashboard is an external snapshot: fetch immediately, then poll at a
  // bounded cadence. The async request, not this effect, performs state updates.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadDashboard()
    const interval = window.setInterval(() => void loadDashboard(), 5 * 60 * 1000)
    return () => window.clearInterval(interval)
  }, [admin, token])

  useEffect(() => {
    if (!token) return
    getSessions(token)
      .then(setSessions)
      .catch((requestError: unknown) => setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to load sessions'))
  }, [token])

  async function revoke(id: string, current: boolean) {
    if (!token || !window.confirm(current ? 'Revoke this session and sign out?' : 'Revoke this administrator session?')) return
    try {
      await revokeSession(token, id)
      if (current) {
        expireSession('Current session revoked')
        return
      }
      setSessions((values) => values.filter((session) => session.id !== id))
      setSessionMessage('Session revoked')
    } catch (requestError) {
      setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to revoke session')
    }
  }

  async function revokeOthers() {
    if (!token || !window.confirm('Revoke all other administrator sessions?')) return
    try {
      await revokeOtherSessions(token)
      setSessions((values) => values.filter((session) => session.current))
      setSessionMessage('All other sessions revoked')
    } catch (requestError) {
      setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to revoke sessions')
    }
  }

  if (!admin) return null

  return (
    <>
      <header className="flex flex-col md:flex-row items-start justify-between gap-5 border-b border-[#28323d] pb-7">
        <div>
          <p className="text-[#4dd0b5] font-mono font-bold text-[11px] tracking-[0.15em]">
            SYSTEM STATUS / {new Date().toISOString().slice(0, 10)}
          </p>
          <h1 className="my-2.5 text-3xl sm:text-4xl md:text-5xl font-bold leading-none tracking-[-0.045em] text-white">
            Operations overview
          </h1>
        </div>
        <span className="border border-[#4dd0b5] text-[#4dd0b5] px-2.5 py-1.5 font-mono font-bold text-[11px] tracking-[0.1em]">
          {dashboard?.environment?.toUpperCase() ?? 'LOADING'}
        </span>
      </header>
      {error ? <p role="alert" className="border-l-[3px] border-l-[#ff786f] bg-[#ff786f12] text-[#ffaaa4] p-3 my-4">{error}</p> : null}
      {!admin.permissions.includes('dashboard.read') ? (
        <section className="min-h-[180px] flex flex-col gap-3 border border-[#28323d] bg-[#10161d] p-6 mt-8">
          <h2 className="text-xl font-bold text-white">Access limited</h2>
          <p className="text-[#99a5b3]">Your administrator account does not have dashboard access.</p>
        </section>
      ) : null}

      <section className="mt-8 border border-[#28323d] bg-[#10161d] p-6" aria-labelledby="sessions-heading">
        <div className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center">
          <div>
            <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">SECURITY</p>
            <h2 id="sessions-heading" className="mt-2 text-xl font-bold text-white">Active sessions</h2>
          </div>
          {sessions.some((session) => !session.current) ? (
            <button onClick={revokeOthers} className="border border-[#ff786f] text-[#ffaaa4] bg-transparent px-4 py-2 cursor-pointer">Revoke all others</button>
          ) : null}
        </div>
        {sessionMessage ? <p role="status" className="text-[#4dd0b5] mt-4">{sessionMessage}</p> : null}
        <div className="grid gap-3 mt-5">
          {sessions.map((session) => (
            <article key={session.id} className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center border border-[#28323d] p-4">
              <div>
                <strong className="text-white">{session.current ? 'Current session' : session.user_agent || 'Unknown device'}</strong>
                <p className="text-[#8493a5] text-sm mt-1">{session.ip_address || 'Unknown IP'} · Started {new Date(session.created_at).toLocaleString()}</p>
              </div>
              <button onClick={() => revoke(session.id, session.current)} className="border border-[#394552] text-[#aeb8c4] bg-transparent px-4 py-2 cursor-pointer">
                {session.current ? 'Revoke and sign out' : 'Revoke'}
              </button>
            </article>
          ))}
        </div>
      </section>
      {dashboardError ? <p role="alert" className="border-l-[3px] border-l-[#ff786f] bg-[#ff786f12] text-[#ffaaa4] p-3 mt-8">{dashboardError}</p> : null}
      {dashboard ? (
        <section className="mt-8 grid gap-6" aria-label="Platform activity">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="m-0 text-xs text-[#8493a5]">All activity windows use UTC calendar boundaries.</p>
            <button type="button" onClick={() => void loadDashboard()} disabled={dashboardLoading} className="border border-[#4dd0b5] px-3 py-2 text-xs font-bold text-[#4dd0b5] disabled:opacity-50">
              {dashboardLoading ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
          <MetricSection title="Current activity" metrics={[
            ['Active players', dashboard.current?.players ?? 0], ['Active rooms', dashboard.current?.rooms ?? 0], ['Games in progress', dashboard.current?.games ?? 0],
          ]} />
          <MetricSection title="Today" window={dashboard.windows?.day} metrics={[
            ['Registrations', dashboard.daily?.registrations ?? 0], ['Players active', dashboard.daily?.players ?? 0], ['Rooms created', dashboard.daily?.rooms ?? 0], ['Games started', dashboard.daily?.games_started ?? 0], ['Games completed', dashboard.daily?.games_completed ?? 0], ['Games abandoned', dashboard.daily?.games_abandoned ?? 0], ['Avg. duration', formatDuration(dashboard.daily?.average_game_duration_seconds ?? 0)],
          ]} />
          <MetricSection title="This month" window={dashboard.windows?.month} metrics={[
            ['Registrations', dashboard.monthly?.registrations ?? 0], ['Players active', dashboard.monthly?.players ?? 0], ['Rooms created', dashboard.monthly?.rooms ?? 0], ['Games started', dashboard.monthly?.games_started ?? 0], ['Games completed', dashboard.monthly?.games_completed ?? 0], ['Games abandoned', dashboard.monthly?.games_abandoned ?? 0], ['Avg. duration', formatDuration(dashboard.monthly?.average_game_duration_seconds ?? 0)],
          ]} />
          <div className="grid gap-4 md:grid-cols-2">
            <article className="border border-[#28323d] bg-[#10161d] p-5">
              <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">SERVICE HEALTH</p>
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <ServiceStatus name="API" status={dashboard.services?.api.status ?? dashboard.status} />
                <ServiceStatus name="WebSocket" status={dashboard.services?.ws.status ?? 'not_configured'} />
              </div>
            </article>
            <article className="border border-[#28323d] bg-[#10161d] p-5">
              <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">AUTHORITATIVE TOOLS</p>
              {dashboard.links?.length ? <div className="mt-4 flex flex-wrap gap-2">{dashboard.links.map((link) => <a key={link.name} href={link.url} target="_blank" rel="noreferrer" className="border border-[#394552] px-3 py-2 text-sm text-[#eafbf7] hover:border-[#4dd0b5]">{link.name}</a>)}</div> : <p className="mt-4 text-sm text-[#8493a5]">No operational links are configured for this environment.</p>}
            </article>
          </div>
        </section>
      ) : null}
    </>
  )
}

function MetricSection({ title, window, metrics }: { title: string; window?: { from: string; to: string }; metrics: Array<[string, number | string]> }) {
  return (
    <section className="border border-[#28323d] bg-[#10161d] p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="m-0 text-xl font-bold text-white">{title}</h2>
        {window ? <small className="text-[#8493a5]">{new Date(window.from).toLocaleDateString()} to {new Date(window.to).toLocaleDateString()}</small> : null}
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-3 xl:grid-cols-6">
        {metrics.map(([label, value]) => <article key={label} className="min-w-0 border border-[#28323d] bg-[#0c1117] p-3"><p className="m-0 text-xs text-[#8493a5]">{label}</p><strong className="mt-2 block truncate text-xl text-white">{value}</strong></article>)}
      </div>
    </section>
  )
}

function ServiceStatus({ name, status }: { name: string; status: string }) {
  const healthy = status === 'ok'
  return <div className="border border-[#28323d] bg-[#0c1117] p-3"><p className="m-0 text-xs text-[#8493a5]">{name}</p><strong className={healthy ? 'mt-2 block text-[#4dd0b5]' : 'mt-2 block text-[#ffaaa4]'}>{status.replace('_', ' ').toUpperCase()}</strong></div>
}

function formatDuration(seconds: number) {
  if (seconds < 60) return `${seconds}s`
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}
