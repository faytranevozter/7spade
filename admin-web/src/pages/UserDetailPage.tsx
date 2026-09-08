import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError } from '../api/client'
import {
  getUser,
  reinstateUser,
  suspendUser,
  updateUserDisplayName,
  type UserDetail,
} from '../api/users'
import { FilterField, SectionHeading } from '../components/InvestigationUI'
import { Notice, ReadOnlyNotice } from '../components/Feedback'
import { formatDateTime } from '../components/formatters'
import {
  UserAccountPanel,
  UserDetailSections,
} from '../components/UserDetailSections'
import { AdminPage, AdminPanel } from '../components/AdminPage'
import { useAuth } from '../hooks/useAuth'
import './UserDetailPage.css'

export function UserDetailPage() {
  const { id = '' } = useParams()
  const { admin, token } = useAuth()
  const [detail, setDetail] = useState<UserDetail | null>(null)
  const [message, setMessage] = useState('')
  const [messageTone, setMessageTone] = useState<'success' | 'error'>('error')
  const [auditEventID, setAuditEventID] = useState('')
  const [reason, setReason] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [displayNameReason, setDisplayNameReason] = useState('')
  const canModerate = admin?.permissions.includes('users.moderate') ?? false

  async function loadUser() {
    if (!token || !id) return
    const next = await getUser(token, id)
    setDetail(next)
    setDisplayName((current) => current || next.user.display_name)
    setMessage('')
  }

  useEffect(() => {
    let cancelled = false
    if (!token || !id) return
    getUser(token, id)
      .then((next) => {
        if (!cancelled) {
          setDetail(next)
          setDisplayName(next.user.display_name)
          setMessage('')
        }
      })
      .catch((error) => {
        if (!cancelled)
          setMessage(
            error instanceof Error ? error.message : 'Failed to load user',
          )
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  async function updateSuspension() {
    if (!detail || !token) return
    const suspending = !detail.user.suspension
    if (
      !window.confirm(
        `${suspending ? 'Suspend' : 'Reinstate'} ${detail.user.display_name} (${detail.user.id})? ${suspending ? 'This immediately revokes active Player credentials.' : 'This restores Player access.'}`,
      )
    )
      return
    try {
      const response = suspending
        ? await suspendUser(
            token,
            detail.user.id,
            reason.trim(),
            expiresAt ? new Date(expiresAt).toISOString() : undefined,
          )
        : await reinstateUser(token, detail.user.id)
      setAuditEventID(response.audit_event_id)
      setMessage(
        `${detail.user.display_name} ${suspending ? 'suspended' : 'reinstated'}.`,
      )
      setMessageTone('success')
      try {
        await loadUser()
      } catch {
        setMessageTone('error')
        setMessage(
          `${detail.user.display_name} ${suspending ? 'suspended' : 'reinstated'}, but the dossier could not be refreshed.`,
        )
      }
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : 'Failed to update access',
      )
      setMessageTone('error')
    }
  }

  async function moderateDisplayName() {
    if (!detail || !token) return
    const nextName = displayName.trim()
    if (
      !window.confirm(
        `Replace ${detail.user.display_name} for ${detail.user.id} with ${nextName}?`,
      )
    )
      return
    try {
      const response = await updateUserDisplayName(
        token,
        detail.user.id,
        nextName,
        displayNameReason.trim(),
        detail.user.version,
      )
      setDetail({ ...detail, user: response.user })
      setDisplayName(response.user.display_name)
      setDisplayNameReason('')
      setAuditEventID(response.audit_event_id)
      setMessage(`Display name updated to ${response.user.display_name}.`)
      setMessageTone('success')
    } catch (error) {
      if (
        error instanceof ApiError &&
        error.status === 409 &&
        error.message === 'User changed since it was loaded'
      ) {
        try {
          const current = await getUser(token, detail.user.id)
          setDetail(current)
          setMessage(
            `This player changed since you loaded it. Current name: ${current.user.display_name}. Review your attempted replacement and submit again.`,
          )
        } catch {
          setMessage(
            'This player changed since you loaded it, but the latest version could not be loaded. Reload before retrying.',
          )
        }
      } else
        setMessage(
          error instanceof Error
            ? error.message
            : 'Failed to update display name',
        )
      setMessageTone('error')
    }
  }

  if (!detail || detail.user.id !== id)
    return (
      <AdminPage>
        <Link
          to="/users"
          className="text-admin-accent hover:text-admin-accent-bright gap-admin-4 text-admin-meta inline-flex items-center font-mono no-underline before:content-['<-']"
        >
          Back to users
        </Link>
        <Notice variant={!detail && message ? 'error' : 'info'}>
          {detail ? 'Loading user...' : message || 'Loading user...'}
        </Notice>
      </AdminPage>
    )
  const { user } = detail
  return (
    <AdminPage labelledBy="user-detail-heading" className="user-dossier">
      <Link
        to="/users"
        className="text-admin-accent hover:text-admin-accent-bright gap-admin-4 text-admin-meta inline-flex items-center font-mono no-underline before:content-['<-']"
      >
        Back to users
      </Link>
      <header className="dossier-identity">
        <span className="text-admin-ink text-admin-section bg-admin-surface-raised grid size-19 shrink-0 place-items-center rounded-full font-semibold">
          {initials(user.display_name)}
        </span>
        <div className="min-w-0 flex-1 wrap-anywhere">
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Player dossier
          </p>
          <h1
            id="user-detail-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-detail-hero leading-none font-medium tracking-[-0.055em]"
          >
            {user.display_name}
          </h1>
          <p className="text-admin-muted text-admin-action m-0">
            @{user.username}
          </p>
          <p className="dossier-id">{user.id}</p>
        </div>
        <span
          className={`text-admin-caption px-admin-10 rounded-full border py-[0.38rem] font-mono uppercase ${user.suspension ? 'text-admin-danger border-admin-danger-border' : 'text-admin-success border-admin-success-border'}`}
        >
          {user.suspension ? 'Suspended' : 'Access active'}
        </span>
      </header>
      <dl className="dossier-metrics">
        <div className="border-admin-border-divider py-admin-15 border-r px-4 max-[480px]:border-r-0 max-[480px]:border-b">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Presence
          </dt>
          <dd
            className={`text-admin-ink-strong text-admin-field mt-1 ${user.online ? 'text-admin-success' : ''}`}
          >
            {user.online ? 'Online' : 'Offline'}
          </dd>
        </div>
        <div className="border-admin-border-divider py-admin-15 border-r px-4 max-[760px]:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Games played
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.stats.games_played ?? 'Not available'}
          </dd>
        </div>
        <div className="border-admin-border-divider py-admin-15 border-r px-4 max-[480px]:border-r-0 max-[480px]:border-b">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Wins
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.stats.wins ?? 'Not available'}
          </dd>
        </div>
        <div className="border-admin-border-divider py-admin-15 border-r px-4 max-[760px]:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Achievements
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.achievements.length}
          </dd>
        </div>
        <div className="border-admin-border-divider py-admin-15 border-0 border-r px-4 max-[480px]:border-r-0 max-[480px]:border-b-0">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Skins owned
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.skins.length}
          </dd>
        </div>
      </dl>
      {message ? (
        <Notice variant={messageTone}>
          {message}{' '}
          {auditEventID && admin?.permissions.includes('audit.read') ? (
            <Link to={`/audit-events/${auditEventID}`}>View audit event</Link>
          ) : null}
        </Notice>
      ) : null}
      <div className="dossier-body">
        <div className="min-w-0">
          <UserDetailSections
            key={id}
            detail={detail}
            permissions={admin?.permissions ?? []}
          />
        </div>
        <aside className="min-w-0">
          <UserAccountPanel
            detail={detail}
            permissions={admin?.permissions ?? []}
          />
          <AdminPanel className="dossier-moderation mt-6 grid gap-4">
            <SectionHeading
              eyebrow="Operator controls"
              title="Moderation"
              id="moderation-heading"
            />
            {canModerate ? (
              <>
                <div className="border-admin-border-divider gap-admin-10 grid border-t pt-4">
                  <h3 className="text-admin-ink-soft text-admin-body m-0">
                    Display name
                  </h3>
                  <FilterField label="Replacement display name">
                    <input
                      value={displayName}
                      maxLength={50}
                      onChange={(event) => setDisplayName(event.target.value)}
                      className="border-admin-border-input bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-150 outline-none"
                    />
                  </FilterField>
                  <FilterField label="Moderation reason">
                    <input
                      value={displayNameReason}
                      onChange={(event) =>
                        setDisplayNameReason(event.target.value)
                      }
                      className="border-admin-border-input bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-150 outline-none"
                    />
                  </FilterField>
                  <button
                    disabled={
                      !displayName.trim() ||
                      !displayNameReason.trim() ||
                      displayName.trim() === user.display_name
                    }
                    onClick={() => void moderateDisplayName()}
                    className="text-admin-danger rounded-admin-input px-admin-10 py-admin-7 text-admin-label border-admin-danger-border w-full cursor-pointer border bg-transparent font-mono"
                  >
                    Replace display name
                  </button>
                </div>
                <div className="border-admin-border-divider gap-admin-10 grid border-t pt-4">
                  <h3 className="text-admin-ink-soft text-admin-body m-0">
                    Player access
                  </h3>
                  {user.suspension ? (
                    <div className="p-admin-10 border-admin-danger bg-admin-danger-bg border-l-2">
                      <strong className="text-admin-danger text-admin-field block">
                        {user.suspension.reason}
                      </strong>
                      <small className="text-admin-muted text-admin-label mt-1 block">
                        {user.suspension.expires_at
                          ? `Until ${formatDateTime(user.suspension.expires_at)}`
                          : 'Indefinite suspension'}
                      </small>
                    </div>
                  ) : (
                    <>
                      <FilterField label="Reason">
                        <input
                          value={reason}
                          onChange={(event) => setReason(event.target.value)}
                          className="border-admin-border-input bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-150 outline-none"
                        />
                      </FilterField>
                      <FilterField label="Expiry (optional)">
                        <input
                          type="datetime-local"
                          value={expiresAt}
                          onChange={(event) => setExpiresAt(event.target.value)}
                          className="border-admin-border-input bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-150 outline-none"
                        />
                      </FilterField>
                    </>
                  )}
                  <button
                    disabled={!user.suspension && !reason.trim()}
                    onClick={() => void updateSuspension()}
                    className={`rounded-admin-input text-admin-label px-admin-10 w-full cursor-pointer border bg-transparent py-[0.55rem] font-mono ${user.suspension ? 'text-admin-success border-admin-success-border' : 'text-admin-danger border-admin-danger-border'}`}
                  >
                    {user.suspension ? 'Reinstate player' : 'Suspend player'}
                  </button>
                </div>
              </>
            ) : (
              <ReadOnlyNotice>
                You can inspect this dossier but cannot moderate the player.
              </ReadOnlyNotice>
            )}
          </AdminPanel>
        </aside>
      </div>
    </AdminPage>
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
