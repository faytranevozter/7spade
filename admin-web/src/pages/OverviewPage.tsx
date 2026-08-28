import { useEffect, useEffectEvent, useState } from 'react'
import { getSessions, revokeOtherSessions, revokeSession, type AdminSession } from '../api/auth'
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
    setLoading(true); setDashboardError('')
    try {
      const result = await getDashboard(token).catch(async (requestError: unknown) => { if (!(requestError instanceof ApiError) || requestError.status !== 401) throw requestError; return getDashboard(await refreshSession()) })
      setDashboard(result)
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 401) { expireSession(requestError.message); return }
      setDashboardError(requestError instanceof Error ? requestError.message : 'Failed to load dashboard')
    } finally { setLoading(false) }
  }

  const refreshDashboard = useEffectEvent(loadDashboard)

  useEffect(() => { if (!token || !admin?.permissions.includes('dashboard.read')) return; const initial = window.setTimeout(() => void refreshDashboard(), 0); const interval = window.setInterval(() => void refreshDashboard(), 300000); return () => { window.clearTimeout(initial); window.clearInterval(interval) } }, [admin, token])
  useEffect(() => { if (!token) return; getSessions(token).then(setSessions).catch((requestError: unknown) => setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to load sessions')) }, [token])

  async function revoke(id: string, current: boolean) { if (!token || !window.confirm(current ? 'Revoke this session and sign out?' : 'Revoke this administrator session?')) return; try { await revokeSession(token, id); if (current) expireSession('Current session revoked'); else { setSessions((values) => values.filter((session) => session.id !== id)); setSessionMessage('Session revoked') } } catch (requestError) { setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to revoke session') } }
  async function revokeOthers() { if (!token || !window.confirm('Revoke all other administrator sessions?')) return; try { await revokeOtherSessions(token); setSessions((values) => values.filter((session) => session.current)); setSessionMessage('All other sessions revoked') } catch (requestError) { setSessionMessage(requestError instanceof Error ? requestError.message : 'Failed to revoke sessions') } }
  if (!admin) return null

  return <section className="overview-page" aria-labelledby="overview-heading">
    <header className="overview-header"><div><p className="eyebrow">System status / {new Date().toISOString().slice(0, 10)}</p><h1 id="overview-heading">Operations overview</h1><p>A current pulse of platform activity, dependency health, and your administrator security posture.</p></div>{admin.permissions.includes('dashboard.read') ? <div className="overview-header-actions"><span className={`environment-badge environment-${dashboard?.environment ?? 'loading'}`}>{dashboard?.environment?.toUpperCase() ?? 'LOADING'}</span><button onClick={() => void loadDashboard()} disabled={loading} className="refresh-action">{loading ? 'Refreshing...' : 'Refresh snapshot'}</button></div> : null}</header>
    {error ? <Notice variant="error">{error}</Notice> : null}{dashboardError ? <Notice variant="error">{dashboardError}</Notice> : null}
    {!admin.permissions.includes('dashboard.read') ? <ReadOnlyNotice title="Dashboard access limited">Your administrator account does not have dashboard access.</ReadOnlyNotice> : null}
    {dashboard ? <section className="pulse-grid" aria-label="Current activity"><PulseMetric label="Active players" value={dashboard.current?.players ?? 0} detail="Connected now" /><PulseMetric label="Active rooms" value={dashboard.current?.rooms ?? 0} detail="Waiting or playing" /><PulseMetric label="Games in progress" value={dashboard.current?.games ?? 0} detail="Authoritative WS state" /><HealthPulse label="API" status={dashboard.services?.api.status ?? dashboard.status} /><HealthPulse label="WebSocket" status={dashboard.services?.ws.status ?? 'not_configured'} /></section> : null}
    <div className="overview-main-grid">
      {dashboard ? <main className="activity-column"><ActivityWindow title="Today" window={dashboard.windows?.day} data={dashboard.daily} /><ActivityWindow title="This month" window={dashboard.windows?.month} data={dashboard.monthly} /><section className="overview-panel"><SectionHeading eyebrow="Authoritative systems" title="Operational tools" id="tools-heading" meta={dashboard.links?.length ?? 0} />{dashboard.links?.length ? <div className="tool-links">{dashboard.links.map((link) => <a key={link.name} href={link.url} target="_blank" rel="noreferrer"><span>{formatLabel(link.name)}</span><small>Open external system</small></a>)}</div> : <EmptyState mark="O" title="No tools configured" description="Operational links have not been configured for this environment." />}</section></main> : <div />}
      <aside className="security-column"><section className="overview-panel session-panel"><div className="session-panel-heading"><SectionHeading eyebrow="Your security" title="Active sessions" id="sessions-heading" meta={sessions.length} />{sessions.some((session) => !session.current) ? <button onClick={() => void revokeOthers()} className="danger-outline-action">Revoke all others</button> : null}</div>{sessionMessage ? <Notice variant="success">{sessionMessage}</Notice> : null}<div className="session-list">{sessions.map((session) => <SessionRow key={session.id} session={session} onRevoke={revoke} />)}</div></section></aside>
    </div>
  </section>
}

function PulseMetric({ label, value, detail }: { label: string; value: number; detail: string }) { return <article className="pulse-card"><span>{label}</span><strong>{value}</strong><small>{detail}</small></article> }
function HealthPulse({ label, status }: { label: string; status: string }) { const healthy = status === 'ok'; return <article className={`pulse-card health-pulse ${healthy ? 'healthy' : 'degraded'}`}><span>{label}</span><strong>{formatLabel(status)}</strong><small>{healthy ? 'Service responding' : 'Needs attention'}</small></article> }
function ActivityWindow({ title, window, data }: { title: string; window?: { from: string; to: string }; data?: Dashboard['daily'] }) { const metrics: Array<[string, number | string]> = [['Registrations', data?.registrations ?? 0], ['Players active', data?.players ?? 0], ['Rooms created', data?.rooms ?? 0], ['Games started', data?.games_started ?? 0], ['Completed', data?.games_completed ?? 0], ['Abandoned', data?.games_abandoned ?? 0], ['Avg. duration', formatDuration(data?.average_game_duration_seconds ?? 0)]]; return <section className="overview-panel"><SectionHeading eyebrow="UTC activity window" title={title} id={`activity-${title}`} meta={window ? `${new Date(window.from).toLocaleDateString()} - ${new Date(window.to).toLocaleDateString()}` : undefined} /><div className="activity-metrics">{metrics.map(([label, value]) => <div key={label}><span>{label}</span><strong>{value}</strong></div>)}</div></section> }
function SessionRow({ session, onRevoke }: { session: AdminSession; onRevoke: (id: string, current: boolean) => Promise<void> }) { return <article className={session.current ? 'session-row current' : 'session-row'}><span className="session-device">{session.current ? 'This device' : 'Device'}</span><div><strong>{session.current ? 'Current session' : session.user_agent || 'Unknown device'}</strong><p>{session.ip_address || 'Unknown IP'}</p><small>Started {formatDateTime(session.created_at)}</small></div><button onClick={() => void onRevoke(session.id, session.current)}>{session.current ? 'Revoke and sign out' : 'Revoke'}</button></article> }
function formatDuration(seconds: number) { return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s` }
