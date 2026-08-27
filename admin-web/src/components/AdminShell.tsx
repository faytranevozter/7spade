import { useEffect, useState } from 'react'
import {
  getAdmins,
  getPermissions,
  getRoles,
  getSessions,
  inviteAdmin,
  revokeOtherSessions,
  revokeSession,
  setAdminRoles,
  setAdminStatus,
  updateRolePermissions,
  type Admin,
  type Permission,
  type AdminSession,
  type Role,
} from '../api/auth'
import { ApiError } from '../api/client'
import { getDashboard, type Dashboard } from '../api/dashboard'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from './AdminBrand'
import { UserInvestigation } from './UserInvestigation'

export function AdminShell() {
  const { admin, token, error, signOut, refreshSession, expireSession } = useAuth()
  const [view, setView] = useState(() => window.location.hash === '#administrators' ? 'administrators' : window.location.hash === '#users' ? 'users' : 'overview')
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [dashboardError, setDashboardError] = useState('')
  const [dashboardLoading, setDashboardLoading] = useState(false)
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [sessionMessage, setSessionMessage] = useState('')
  const [adminsList, setAdminsList] = useState<Admin[]>([])
  const [rolesList, setRolesList] = useState<Role[]>([])
  const [permissionsList, setPermissionsList] = useState<Permission[]>([])
  const [adminMessage, setAdminMessage] = useState('')
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRoleId, setInviteRoleId] = useState('')
  const [generatedInviteLink, setGeneratedInviteLink] = useState('')

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

  useEffect(() => {
    if (!token || view !== 'administrators' || !admin?.permissions.includes('admins.read')) return
    getAdmins(token)
      .then(setAdminsList)
      .catch((requestError: unknown) => setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to load administrators'))
    getRoles(token)
      .then((roles) => {
        setRolesList(roles)
        if (roles.length > 0) setInviteRoleId(roles[0].id)
      })
      .catch(() => {})
    if (admin.permissions.includes('admins.manage')) {
      getPermissions(token).then(setPermissionsList).catch(() => {})
    }
  }, [admin, token, view])

  useEffect(() => {
    function syncView() {
        setView(window.location.hash === '#administrators' ? 'administrators' : window.location.hash === '#users' ? 'users' : 'overview')
    }
    window.addEventListener('hashchange', syncView)
    return () => window.removeEventListener('hashchange', syncView)
  }, [])

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault()
    if (!token || !inviteEmail || !inviteRoleId) return
    setAdminMessage('')
    try {
      const res = await inviteAdmin(token, inviteEmail, inviteRoleId)
      setGeneratedInviteLink(`Token: ${res.token}`)
      setAdminMessage(`Invitation created for ${res.invitation.email}`)
      setInviteEmail('')
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to invite administrator')
    }
  }

  async function handleToggleStatus(targetAdmin: Admin) {
    if (!token) return
    const nextStatus = targetAdmin.status === 'active' ? 'disabled' : 'active'
    if (!window.confirm(`Are you sure you want to ${nextStatus === 'disabled' ? 'disable' : 'activate'} ${targetAdmin.email}?`)) return
    try {
      await setAdminStatus(token, targetAdmin.id, nextStatus)
      setAdminsList((list) => list.map((a) => a.id === targetAdmin.id ? { ...a, status: nextStatus } : a))
      setAdminMessage(`Administrator ${targetAdmin.email} is now ${nextStatus}`)
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update administrator status')
    }
  }

  async function handleRoleChange(targetAdmin: Admin, roleId: string) {
    if (!token) return
    const roleIds = [roleId, ...(targetAdmin.roles?.slice(1).map((role) => role.id) ?? [])]
    try {
      await setAdminRoles(token, targetAdmin.id, roleIds)
      const targetRole = rolesList.find((r) => r.id === roleId)
      setAdminsList((list) => list.map((a) => a.id === targetAdmin.id ? { ...a, roles: targetRole ? [targetRole, ...(a.roles?.slice(1) ?? [])] : a.roles } : a))
      setAdminMessage('Administrator role updated')
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update administrator role')
    }
  }

  function toggleRolePermission(roleId: string, permission: string) {
    setRolesList((roles) => roles.map((role) => role.id !== roleId ? role : {
      ...role,
      permissions: role.permissions.includes(permission)
        ? role.permissions.filter((value) => value !== permission)
        : [...role.permissions, permission],
    }))
  }

  async function saveRolePermissions(role: Role) {
    if (!token) return
    try {
      await updateRolePermissions(token, role.id, role.permissions)
      if (admin?.roles?.some((assignedRole) => assignedRole.id === role.id)) {
        expireSession('Your permissions changed. Sign in again to continue.')
        return
      }
      setAdminMessage(`${role.name[0].toUpperCase()}${role.name.slice(1)} permissions updated`)
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update role permissions')
    }
  }

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
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-[260px_1fr]">
      <aside className="static md:sticky top-0 w-full md:h-screen flex flex-col border-b md:border-b-0 md:border-r border-[#28323d] bg-[#0c1117] p-6 md:p-4.5">
        <AdminBrand subtitle="Seven Spade operations" />
        <nav aria-label="Admin navigation" className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-1 gap-1.5 mt-6 md:mt-12">
          {admin.permissions.includes('dashboard.read') ? (
            <a
              className="text-[#eafbf7] border-l-2 border-[#4dd0b5] bg-[#4dd0b5]/5 py-2.5 px-3 no-underline focus-visible:outline-2 focus-visible:outline-[#4dd0b5]"
              href="#overview"
            >
              Overview
            </a>
          ) : null}
          {admin.permissions.includes('admins.read') ? (
            <a
              className="text-[#7f8c9b] hover:text-white border-l-2 border-transparent py-2.5 px-3 no-underline"
              href="#administrators"
            >
              Administrators
            </a>
          ) : null}
          {admin.permissions.includes('users.read') ? (
            <a className="text-[#7f8c9b] hover:text-white border-l-2 border-transparent py-2.5 px-3 no-underline" href="#users">Users</a>
          ) : null}
          {admin.permissions.includes('rooms.read') ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Rooms</span>
          ) : null}
          {admin.permissions.includes('games.read') ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Games</span>
          ) : null}
          {admin.permissions.some((permission) =>
            ['seasons.read', 'events.read', 'achievements.read', 'skins.read'].includes(permission),
          ) ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Content</span>
          ) : null}
          {admin.permissions.includes('audit.read') ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Audit</span>
          ) : null}
        </nav>
        <div className="mt-6 md:mt-auto grid gap-1.5 border-t border-[#28323d] pt-4.5">
          <small className="text-[#8493a5]">Signed in as</small>
          <strong className="text-white font-bold">{admin.display_name}</strong>
          <button
            onClick={signOut}
            className="mt-2 border border-[#394552] bg-transparent text-[#aeb8c4] p-2.5 cursor-pointer hover:bg-white/5 transition-colors focus-visible:outline-2 focus-visible:outline-[#4dd0b5]"
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="p-6 md:p-12 lg:p-18">
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

        {view === 'users' && admin.permissions.includes('users.read') && token ? <UserInvestigation token={token} canReadSensitive={admin.permissions.includes('users.sensitive.read')} /> : null}

        {view === 'administrators' && admin.permissions.includes('admins.read') ? (
          <section id="administrators" className="mt-8 border border-[#28323d] bg-[#10161d] p-6" aria-labelledby="admins-heading">
            <div className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center">
              <div>
                <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">ACCESS CONTROL</p>
                <h2 id="admins-heading" className="mt-2 text-xl font-bold text-white">Administrators</h2>
              </div>
            </div>

            {admin.permissions.includes('admins.manage') ? (
              <form onSubmit={handleInvite} className="mt-5 p-4 border border-[#28323d] bg-[#0c1117] flex flex-col sm:flex-row gap-3 items-end">
                <div className="flex-1 w-full">
                  <label htmlFor="invite-email" className="block text-xs font-mono text-[#8493a5] mb-1">Invite Email</label>
                  <input
                    id="invite-email"
                    type="email"
                    required
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    placeholder="admin@example.com"
                    className="w-full bg-[#10161d] border border-[#28323d] text-white px-3 py-2 text-sm focus:outline-none focus:border-[#4dd0b5]"
                  />
                </div>
                <div className="w-full sm:w-48">
                  <label htmlFor="invite-role" className="block text-xs font-mono text-[#8493a5] mb-1">Role</label>
                  <select
                    id="invite-role"
                    value={inviteRoleId}
                    onChange={(e) => setInviteRoleId(e.target.value)}
                    className="w-full bg-[#10161d] border border-[#28323d] text-white px-3 py-2 text-sm focus:outline-none focus:border-[#4dd0b5]"
                  >
                    {rolesList.map((r) => (
                      <option key={r.id} value={r.id}>{r.name}</option>
                    ))}
                  </select>
                </div>
                <button
                  type="submit"
                  className="bg-[#4dd0b5] text-[#0c1117] font-bold px-4 py-2 text-sm hover:bg-[#3dbca2] cursor-pointer"
                >
                  Send Invite
                </button>
              </form>
            ) : null}

            {generatedInviteLink ? (
              <div className="mt-3 p-3 bg-[#4dd0b5]/10 border border-[#4dd0b5] text-[#4dd0b5] text-xs font-mono">
                {generatedInviteLink}
              </div>
            ) : null}

            {adminMessage ? <p role="status" className="text-[#4dd0b5] mt-4">{adminMessage}</p> : null}

            {admin.permissions.includes('admins.manage') ? (
              <div className="grid gap-3 mt-5" aria-label="Role permissions">
                {rolesList.map((role) => (
                  <fieldset key={role.id} className="border border-[#28323d] p-4">
                    <legend className="px-2 font-bold text-white">{role.name}</legend>
                    <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-2">
                      {permissionsList.map((permission) => (
                        <label key={permission.name} className="flex items-start gap-2 text-sm text-[#aeb8c4]">
                          <input
                            type="checkbox"
                            aria-label={`${permission.name} for ${role.name}`}
                            checked={role.permissions.includes(permission.name)}
                            onChange={() => toggleRolePermission(role.id, permission.name)}
                          />
                          <span><strong className="text-white">{permission.name}</strong><br />{permission.description}</span>
                        </label>
                      ))}
                    </div>
                    <button
                      type="button"
                      onClick={() => saveRolePermissions(role)}
                      className="mt-3 border border-[#4dd0b5] text-[#4dd0b5] bg-transparent px-3 py-1.5 text-xs cursor-pointer"
                    >
                      Save {role.name} permissions
                    </button>
                  </fieldset>
                ))}
              </div>
            ) : null}

            <div className="grid gap-3 mt-5">
              {adminsList.map((adm) => (
                <article key={adm.id} className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center border border-[#28323d] p-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <strong className="text-white">{adm.display_name}</strong>
                      <span className={`text-[10px] font-mono px-2 py-0.5 border ${adm.status === 'active' ? 'border-[#4dd0b5] text-[#4dd0b5]' : 'border-[#ff786f] text-[#ffaaa4]'}`}>
                        {adm.status.toUpperCase()}
                      </span>
                    </div>
                    <p className="text-[#8493a5] text-sm mt-1">{adm.email}</p>
                    <div className="flex flex-wrap gap-1 mt-2">
                      {adm.roles?.map((r) => (
                        <span key={r.id} className="text-xs bg-[#28323d] text-[#eafbf7] px-2 py-0.5">
                          {r.name}
                        </span>
                      ))}
                    </div>
                  </div>

                  {admin.permissions.includes('admins.manage') && adm.id !== admin.id ? (
                    <div className="flex items-center gap-3">
                      <select
                        aria-label={`Role for ${adm.display_name}`}
                        value={adm.roles?.[0]?.id ?? ''}
                        onChange={(e) => handleRoleChange(adm, e.target.value)}
                        className="bg-[#0c1117] border border-[#28323d] text-white px-2 py-1 text-xs focus:outline-none focus:border-[#4dd0b5]"
                      >
                        {rolesList.map((r) => (
                          <option key={r.id} value={r.id}>{r.name}</option>
                        ))}
                      </select>
                      <button
                        onClick={() => handleToggleStatus(adm)}
                        className={`border px-3 py-1 text-xs cursor-pointer ${adm.status === 'active' ? 'border-[#ff786f] text-[#ffaaa4]' : 'border-[#4dd0b5] text-[#4dd0b5]'}`}
                      >
                        {adm.status === 'active' ? 'Disable' : 'Activate'}
                      </button>
                    </div>
                  ) : null}
                </article>
              ))}
            </div>
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
      </main>
    </div>
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
