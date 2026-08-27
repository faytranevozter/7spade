import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { getDashboard, type Dashboard } from '../api/dashboard'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from './AdminBrand'

export function AdminShell() {
  const { admin, token, error, signOut, refreshSession, expireSession } = useAuth()
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)

  useEffect(() => {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
    let cancelled = false
    getDashboard(token)
      .catch(async (requestError: unknown) => {
        if (!(requestError instanceof ApiError) || requestError.status !== 401) throw requestError
        const freshToken = await refreshSession()
        return getDashboard(freshToken)
      })
      .then((result) => { if (!cancelled) setDashboard(result) })
      .catch((requestError: unknown) => {
        if (!cancelled) expireSession(requestError instanceof Error ? requestError.message : 'Session expired')
      })
    return () => { cancelled = true }
  }, [admin, token, expireSession, refreshSession])

  if (!admin) return null

  return (
    <div className="app-shell">
      <aside>
        <AdminBrand subtitle="Seven Spade operations" />
        <nav aria-label="Admin navigation">
          {admin.permissions.includes('dashboard.read') ? <a className="active" href="#overview">Overview</a> : null}
          {admin.permissions.includes('users.read') ? <span>Users</span> : null}
          {admin.permissions.includes('rooms.read') ? <span>Rooms</span> : null}
          {admin.permissions.includes('games.read') ? <span>Games</span> : null}
          {admin.permissions.some((permission) => ['seasons.read', 'events.read', 'achievements.read', 'skins.read'].includes(permission)) ? <span>Content</span> : null}
          {admin.permissions.includes('audit.read') ? <span>Audit</span> : null}
        </nav>
        <div className="identity">
          <small>Signed in as</small>
          <strong>{admin.display_name}</strong>
          <button onClick={signOut}>Sign out</button>
        </div>
      </aside>
      <main>
        <header>
          <div><p className="eyebrow">SYSTEM STATUS / {new Date().toISOString().slice(0, 10)}</p><h1>Operations overview</h1></div>
          <span className="environment">{dashboard?.environment.toUpperCase() ?? 'LOADING'}</span>
        </header>
        {error ? <p role="alert" className="error">{error}</p> : null}
        {!admin.permissions.includes('dashboard.read') ? <section className="panel"><h2>Access limited</h2><p>Your administrator account does not have dashboard access.</p></section> : null}
        {dashboard ? <section className="grid"><article className="panel primary"><p>ADMIN API</p><strong>{dashboard.status.toUpperCase()}</strong><small>Authentication and authorization online</small></article><article className="panel"><p>LIVE OPERATIONS</p><strong>Foundation active</strong><small>User and Room monitoring arrives in the next vertical slice.</small></article><article className="panel"><p>SECURITY</p><strong>{admin.permissions.length}</strong><small>effective permission{admin.permissions.length === 1 ? '' : 's'}</small></article></section> : null}
      </main>
    </div>
  )
}
