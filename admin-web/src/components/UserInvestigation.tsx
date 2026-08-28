import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchUsers, type User } from '../api/users'
import { EmptyState, FilterField, Pagination, SectionHeading } from './InvestigationUI'
import { Notice } from './Feedback'
import { formatDateTime } from './formatters'

export function UserInvestigation({ token, canReadSensitive }: { token: string; canReadSensitive: boolean }) {
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    const request = window.setTimeout(() => {
      setLoading(true)
      searchUsers(token, query, pageSize, offset).then((page) => {
        if (!cancelled) { setUsers(page.users ?? []); setMessage('') }
      }).catch((error: unknown) => {
        if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to search users')
      }).finally(() => { if (!cancelled) setLoading(false) })
    }, 0)
    return () => { cancelled = true; window.clearTimeout(request) }
  }, [offset, query, token])

  const online = users.filter((user) => user.online).length
  const suspended = users.filter((user) => user.suspension).length

  return <section className="user-investigation-page" aria-labelledby="users-heading">
    <header className="user-investigation-header">
      <div><p className="eyebrow">Operations / Player investigations</p><h1 id="users-heading">Users</h1><p>Locate player identities, assess access and presence, then open a complete progression and moderation dossier.</p></div>
      <div className="user-header-stats"><div><strong>{users.length}</strong><span>On this page</span></div><div><strong>{online}</strong><span>Online now</span></div><div><strong>{suspended}</strong><span>Suspended</span></div></div>
    </header>

    <section className="user-directory-panel" aria-labelledby="user-directory-heading">
      <div className="user-directory-toolbar">
        <SectionHeading eyebrow="Identity directory" title="Player records" id="user-directory-heading" meta={`Page ${Math.floor(offset / pageSize) + 1}`} />
        <Pagination offset={offset} pageSize={pageSize} itemCount={users.length} loading={loading} onOffsetChange={setOffset} label="User result pages" />
      </div>
      <div className="user-search-row">
        <FilterField label={canReadSensitive ? 'Search ID, username, display name, or email' : 'Search ID, username, or display name'}><input value={query} onChange={(event) => { setQuery(event.target.value); setOffset(0) }} placeholder="Search player records..." className="game-input" /></FilterField>
        <div className="privacy-scope"><span>{canReadSensitive ? 'Sensitive read enabled' : 'Standard redaction'}</span><small>{canReadSensitive ? 'Normalized email may appear in results.' : 'Email remains redacted by policy.'}</small></div>
      </div>
      {message ? <Notice variant="error">{message}</Notice> : null}
      {!loading && users.length === 0 ? <EmptyState mark="U" title="No users found" description="Try a different username, display name, or identifier." /> : null}
      <div className="user-result-list">{users.map((user) => <UserResult key={user.id} user={user} />)}</div>
    </section>
  </section>
}

function UserResult({ user }: { user: User }) {
  return <Link to={`/users/${user.id}`} className="user-result-row" aria-label={`${user.display_name} @${user.username}`}>
    <span className="user-avatar">{initials(user.display_name)}</span>
    <div className="user-result-identity"><div><strong>{user.display_name}</strong><span>@{user.username}</span></div><p>{user.id}</p></div>
    <div className="user-access-state"><span className={user.suspension ? 'access-suspended' : 'access-active'}>{user.suspension ? 'Suspended' : 'Access active'}</span>{user.suspension ? <small>{user.suspension.reason}</small> : <small>Created {formatDateTime(user.created_at)}</small>}</div>
    <div className="user-presence"><span className={user.online ? 'presence-dot online' : 'presence-dot'} /><div><strong>{user.online ? 'Online' : 'Offline'}</strong>{user.email ? <small>{user.email}</small> : null}</div></div>
    <span className="result-arrow" aria-hidden="true">Inspect</span>
  </Link>
}

function initials(name: string) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || '?'
}
