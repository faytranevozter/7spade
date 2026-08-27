import { FormEvent, useEffect, useState } from 'react'
import { apiRequest, login, logout, refresh, type Admin } from './api'

type Dashboard = { status: string; environment: string }

export default function App() {
  const [admin, setAdmin] = useState<Admin | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    refresh().then((result) => { setAdmin(result.admin); setToken(result.access_token) }).catch(() => {}).finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (!token || !admin?.permissions.includes('dashboard.read')) return
    apiRequest<Dashboard>('/dashboard', token).then(setDashboard).catch((requestError: Error) => setError(requestError.message))
  }, [admin, token])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const data = new FormData(event.currentTarget)
    try {
      const result = await login(String(data.get('email')), String(data.get('password')))
      setAdmin(result.admin)
      setToken(result.access_token)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : 'Sign in failed')
    }
  }

  async function signOut() {
    await logout().catch(() => {})
    setAdmin(null)
    setToken(null)
    setDashboard(null)
  }

  if (loading) return <main className="loading">Checking administrator session...</main>
  if (!admin) return <Login onSubmit={submit} error={error} />

  return (
    <div className="app-shell">
      <aside>
        <div className="brand"><span className="mark">7S</span><div><strong>CONTROL ROOM</strong><small>Seven Spade operations</small></div></div>
        <nav aria-label="Admin navigation">
          {admin.permissions.includes('dashboard.read') ? <a className="active" href="#overview">Overview</a> : null}
          {admin.permissions.includes('users.read') ? <span>Users</span> : null}
          {admin.permissions.includes('rooms.read') ? <span>Rooms</span> : null}
          {admin.permissions.includes('games.read') ? <span>Games</span> : null}
          {admin.permissions.some((permission) => ['seasons.read', 'events.read', 'achievements.read', 'skins.read'].includes(permission)) ? <span>Content</span> : null}
          {admin.permissions.includes('audit.read') ? <span>Audit</span> : null}
        </nav>
        <div className="identity"><small>Signed in as</small><strong>{admin.display_name}</strong><button onClick={signOut}>Sign out</button></div>
      </aside>
      <main>
        <header><div><p className="eyebrow">SYSTEM STATUS / {new Date().toISOString().slice(0, 10)}</p><h1>Operations overview</h1></div><span className="environment">{dashboard?.environment.toUpperCase() ?? 'LOADING'}</span></header>
        {error ? <p role="alert" className="error">{error}</p> : null}
        {!admin.permissions.includes('dashboard.read') ? <section className="panel"><h2>Access limited</h2><p>Your administrator account does not have dashboard access.</p></section> : null}
        {dashboard ? <section className="grid"><article className="panel primary"><p>ADMIN API</p><strong>{dashboard.status.toUpperCase()}</strong><small>Authentication and authorization online</small></article><article className="panel"><p>LIVE OPERATIONS</p><strong>Foundation active</strong><small>User and Room monitoring arrives in the next vertical slice.</small></article><article className="panel"><p>SECURITY</p><strong>{admin.permissions.length}</strong><small>effective permission{admin.permissions.length === 1 ? '' : 's'}</small></article></section> : null}
      </main>
    </div>
  )
}

function Login({ onSubmit, error }: { onSubmit: (event: FormEvent<HTMLFormElement>) => void; error: string }) {
  return <main className="login-page"><section className="login-card"><div className="brand"><span className="mark">7S</span><div><strong>CONTROL ROOM</strong><small>Restricted operations console</small></div></div><p className="eyebrow">AUTHORIZED PERSONNEL ONLY</p><h1>Admin sign in</h1><p className="lede">Use your dedicated administrator identity. Player credentials are not accepted.</p><form onSubmit={onSubmit}><label>Email<input name="email" type="email" autoComplete="username" required /></label><label>Password<input name="password" type="password" autoComplete="current-password" required /></label>{error ? <p role="alert" className="error">{error}</p> : null}<button type="submit">Sign in</button></form></section></main>
}
