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

const inputClass =
  'w-full min-w-0 rounded-[7px] border border-[#f4ead5]/15 bg-[#0d1a12] px-3 py-[0.7rem] text-[0.8rem] text-[#fafaf8] outline-none transition-[border-color,box-shadow,background] duration-[120ms] placeholder:text-[#5f665e] focus:border-[#c9922b] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43_/_14%)]'

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
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Operations / Player investigations
          </p>
          <h1
            id="users-heading"
            className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            Users
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Locate player identities, assess access and presence, then open a
            complete progression and moderation dossier.
          </p>
        </div>
        <div className="border-admin-ink/11 bg-admin-surface/80 [&_span]:text-admin-muted-subtle [&_strong]:text-admin-accent-bright [&>div]:border-admin-ink/9 grid min-w-82.5 grid-cols-3 overflow-hidden rounded-xl border max-[760px]:min-w-0 max-[480px]:grid-cols-1 [&_span]:mt-[0.2rem] [&_span]:block [&_span]:text-[0.6rem] [&_strong]:block [&_strong]:font-mono [&_strong]:text-[1.25rem] [&_strong]:font-medium [&>div]:border-r [&>div]:p-[0.85rem] max-[480px]:[&>div]:border-r-0 max-[480px]:[&>div]:border-b [&>div:last-child]:border-r-0 max-[480px]:[&>div:last-child]:border-b-0">
          <div>
            <strong>{users.length}</strong>
            <span>On this page</span>
          </div>
          <div>
            <strong>{online}</strong>
            <span>Online now</span>
          </div>
          <div>
            <strong>{suspended}</strong>
            <span>Suspended</span>
          </div>
        </div>
      </header>

      <section
        className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5"
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
        <div className="border-admin-ink/8 bg-admin-canvas/60 my-4 flex items-center justify-between gap-4 rounded-[9px] border p-4 max-[760px]:flex-col max-[760px]:items-stretch [&>label]:flex-1">
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
              className={inputClass}
            />
          </FilterField>
          <div className="border-admin-ink/9 [&>small]:text-admin-muted-subtle min-w-45 border-l pl-4 max-[760px]:border-t max-[760px]:border-l-0 max-[760px]:px-0 max-[760px]:pt-4 [&>small]:mt-[0.2rem] [&>small]:block [&>small]:text-[0.58rem] [&>span]:block [&>span]:text-[0.65rem] [&>span]:text-[#e0b45e]">
            <span>
              {canReadSensitive
                ? 'Sensitive read enabled'
                : 'Standard redaction'}
            </span>
            <small>
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
        <div className="grid gap-[0.55rem]">
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
      className="border-admin-ink/9 hover:border-admin-accent/35 grid grid-cols-[44px_minmax(220px,1fr)_minmax(150px,0.6fr)_minmax(130px,0.5fr)_auto] items-center gap-[0.85rem] rounded-[9px] border p-[0.8rem] text-inherit no-underline transition-[background,border-color] duration-120 hover:bg-white/2 max-[1100px]:grid-cols-[44px_minmax(200px,1fr)_minmax(130px,0.6fr)_auto] max-[760px]:grid-cols-[40px_minmax(0,1fr)]"
      aria-label={`${user.display_name} @${user.username}`}
    >
      <span className="text-admin-ink grid size-10 place-items-center rounded-full bg-[#235c36] text-[0.7rem] font-semibold">
        {initials(user.display_name)}
      </span>
      <div className="[&_span]:text-admin-muted [&_strong]:text-admin-ink [&_p]:mt-1 [&_p]:mb-0 [&_p]:max-w-75 [&_p]:overflow-hidden [&_p]:font-mono [&_p]:text-[0.55rem] [&_p]:text-ellipsis [&_p]:whitespace-nowrap [&_p]:text-[#60645e] [&_span]:text-[0.68rem] [&_strong]:text-[0.82rem] [&>div]:flex [&>div]:items-baseline [&>div]:gap-2">
        <div>
          <strong>{user.display_name}</strong>
          <span>@{user.username}</span>
        </div>
        <p>{user.id}</p>
      </div>
      <div className="[&_small]:text-admin-muted-subtle grid gap-[0.2rem] max-[760px]:col-start-2 [&_small]:max-w-45 [&_small]:overflow-hidden [&_small]:text-[0.58rem] [&_small]:text-ellipsis [&_small]:whitespace-nowrap [&_span]:text-[0.65rem]">
        <span
          className={
            user.suspension ? 'text-admin-danger' : 'text-admin-success'
          }
        >
          {user.suspension ? 'Suspended' : 'Access active'}
        </span>
        {user.suspension ? (
          <small>{user.suspension.reason}</small>
        ) : (
          <small>Created {formatDateTime(user.created_at)}</small>
        )}
      </div>
      <div className="[&_small]:text-admin-muted-subtle grid grid-cols-[8px_1fr] items-center gap-[0.2rem] max-[1100px]:col-start-2 max-[760px]:col-start-2 [&_small]:max-w-45 [&_small]:overflow-hidden [&_small]:text-[0.58rem] [&_small]:text-ellipsis [&_small]:whitespace-nowrap [&_strong]:text-[0.67rem] [&_strong]:text-[#d9d4c8]">
        <span
          className={
            user.online
              ? 'size-1.5 rounded-full bg-[#56b875] shadow-[0_0_0_3px_rgb(45_122_70/14%)]'
              : 'size-1.5 rounded-full bg-[#665d57]'
          }
        />
        <div>
          <strong>{user.online ? 'Online' : 'Offline'}</strong>
          {user.email ? <small>{user.email}</small> : null}
        </div>
      </div>
      <span
        className="text-admin-accent font-mono text-[0.65rem] uppercase max-[1100px]:col-start-4 max-[1100px]:row-start-1 max-[760px]:hidden"
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
