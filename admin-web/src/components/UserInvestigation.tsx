import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchUsers, type User } from '../api/users'
import {
  EmptyState,
  FilterField,
  Pagination,
  SectionHeading,
} from './InvestigationUI'
import { Notice } from './Feedback'
import { formatDateTime } from './formatters'

export function UserInvestigation({
  token,
  canReadSensitive,
}: {
  token: string
  canReadSensitive: boolean
}) {
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
      searchUsers(token, query, pageSize, offset)
        .then((page) => {
          if (!cancelled) {
            setUsers(page.users ?? [])
            setMessage('')
          }
        })
        .catch((error: unknown) => {
          if (!cancelled)
            setMessage(
              error instanceof Error ? error.message : 'Failed to search users',
            )
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }, 0)
    return () => {
      cancelled = true
      window.clearTimeout(request)
    }
  }, [offset, query, token])

  const online = users.filter((user) => user.online).length
  const suspended = users.filter((user) => user.suspension).length

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="users-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Operations / Player investigations
          </p>
          <h1
            id="users-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-investigation-hero font-medium"
          >
            Users
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Locate player identities, assess access and presence, then open a
            complete progression and moderation dossier.
          </p>
        </div>
        <div className="border-admin-ink/11 bg-admin-surface/80 grid min-w-82.5 grid-cols-3 overflow-hidden rounded-xl border max-[760px]:min-w-0 max-[480px]:grid-cols-1">
          <div className="border-admin-ink/9 p-admin-14 border-r max-[480px]:border-r-0 max-[480px]:border-b">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {users.length}
            </strong>
            <span className="text-admin-muted-subtle mt-admin-2 text-admin-label block">
              On this page
            </span>
          </div>
          <div className="border-admin-ink/9 p-admin-14 border-r max-[480px]:border-r-0 max-[480px]:border-b">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {online}
            </strong>
            <span className="text-admin-muted-subtle mt-admin-2 text-admin-label block">
              Online now
            </span>
          </div>
          <div className="p-admin-14 max-[480px]:border-b-0">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {suspended}
            </strong>
            <span className="text-admin-muted-subtle mt-admin-2 text-admin-label block">
              Suspended
            </span>
          </div>
        </div>
      </header>

      <section
        className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel mt-6 border p-5"
        aria-labelledby="user-directory-heading"
      >
        <div className="flex items-center justify-between gap-4 max-[760px]:flex-col max-[760px]:items-stretch">
          <SectionHeading
            eyebrow="Identity directory"
            title="Player records"
            id="user-directory-heading"
            meta={`Page ${Math.floor(offset / pageSize) + 1}`}
          />
          <Pagination
            offset={offset}
            pageSize={pageSize}
            itemCount={users.length}
            loading={loading}
            onOffsetChange={setOffset}
            label="User result pages"
          />
        </div>
        <div className="border-admin-ink/8 bg-admin-canvas/60 rounded-admin-rule my-4 flex items-center justify-between gap-4 border p-4 max-[760px]:flex-col max-[760px]:items-stretch">
          <div className="flex-1">
            <FilterField
              label={
                canReadSensitive
                  ? 'Search ID, username, display name, or email'
                  : 'Search ID, username, or display name'
              }
            >
              <input
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value)
                  setOffset(0)
                }}
                placeholder="Search player records..."
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
              />
            </FilterField>
          </div>
          <div className="border-admin-ink/9 min-w-45 border-l pl-4 max-[760px]:border-t max-[760px]:border-l-0 max-[760px]:px-0 max-[760px]:pt-4">
            <span className="text-admin-meta text-admin-warning block">
              {canReadSensitive
                ? 'Sensitive read enabled'
                : 'Standard redaction'}
            </span>
            <small className="text-admin-muted-subtle mt-admin-2 text-admin-xs block">
              {canReadSensitive
                ? 'Normalized email may appear in results.'
                : 'Email remains redacted by policy.'}
            </small>
          </div>
        </div>
        {message ? <Notice variant="error">{message}</Notice> : null}
        {!loading && users.length === 0 ? (
          <EmptyState
            mark="U"
            title="No users found"
            description="Try a different username, display name, or identifier."
          />
        ) : null}
        <div className="gap-admin-7 grid">
          {users.map((user) => (
            <UserResult key={user.id} user={user} />
          ))}
        </div>
      </section>
    </section>
  )
}

function UserResult({ user }: { user: User }) {
  return (
    <Link
      to={`/users/${user.id}`}
      className="border-admin-ink/9 hover:border-admin-accent/35 gap-admin-14 rounded-admin-rule p-admin-13 grid grid-cols-[44px_minmax(220px,1fr)_minmax(150px,0.6fr)_minmax(130px,0.5fr)_auto] items-center border text-inherit no-underline transition-[background,border-color] duration-120 hover:bg-white/2 max-[1100px]:grid-cols-[44px_minmax(200px,1fr)_minmax(130px,0.6fr)_auto] max-[760px]:grid-cols-[40px_minmax(0,1fr)]"
      aria-label={`${user.display_name} @${user.username}`}
    >
      <span className="text-admin-ink bg-admin-success-bg text-admin-control grid size-10 place-items-center rounded-full font-semibold">
        {initials(user.display_name)}
      </span>
      <div>
        <div className="flex items-baseline gap-2">
          <strong className="text-admin-ink text-admin-value">
            {user.display_name}
          </strong>
          <span className="text-admin-muted text-admin-note">
            @{user.username}
          </span>
        </div>
        <p className="text-admin-xs text-admin-muted-subtle mt-1 mb-0 max-w-75 overflow-hidden font-mono text-ellipsis whitespace-nowrap">
          {user.id}
        </p>
      </div>
      <div className="gap-admin-2 grid max-[760px]:col-start-2">
        <span
          className={`text-admin-row ${user.suspension ? 'text-admin-danger' : 'text-admin-success'}`}
        >
          {user.suspension ? 'Suspended' : 'Access active'}
        </span>
        {user.suspension ? (
          <small className="text-admin-muted-subtle text-admin-xs max-w-45 overflow-hidden text-ellipsis whitespace-nowrap">
            {user.suspension.reason}
          </small>
        ) : (
          <small className="text-admin-muted-subtle text-admin-xs max-w-45 overflow-hidden text-ellipsis whitespace-nowrap">
            Created {formatDateTime(user.created_at)}
          </small>
        )}
      </div>
      <div className="gap-admin-2 grid grid-cols-[8px_1fr] items-center max-[1100px]:col-start-2 max-[760px]:col-start-2">
        <span
          className={
            user.online
              ? 'bg-admin-success size-1.5 rounded-full shadow-[0_0_0_3px_rgb(45_122_70/14%)]'
              : 'bg-admin-muted-subtle size-1.5 rounded-full'
          }
        />
        <div>
          <strong className="text-admin-small text-admin-ink-soft">
            {user.online ? 'Online' : 'Offline'}
          </strong>
          {user.email ? (
            <small className="text-admin-muted-subtle text-admin-xs max-w-45 overflow-hidden text-ellipsis whitespace-nowrap">
              {user.email}
            </small>
          ) : null}
        </div>
      </div>
      <span
        className="text-admin-accent text-admin-meta font-mono uppercase max-[1100px]:col-start-4 max-[1100px]:row-start-1 max-[760px]:hidden"
        aria-hidden="true"
      >
        Inspect
      </span>
    </Link>
  )
}

function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('') || '?'
  )
}
