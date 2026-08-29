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
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function UserDetailPage() {
  const { id = '' } = useParams()
  const { admin, token } = useAuth()
  const [detail, setDetail] = useState<UserDetail | null>(null)
  const [message, setMessage] = useState('Loading user...')
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
      <section className="mx-auto w-full max-w-360">
        <Link
          to="/users"
          className="text-admin-accent hover:text-admin-accent-bright gap-admin-4 text-admin-meta inline-flex items-center font-mono no-underline before:content-['<-']"
        >
          Back to users
        </Link>
        <Notice variant="info" role="alert">
          {detail ? 'Loading user...' : message}
        </Notice>
      </section>
    )
  const { user } = detail
  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="user-detail-heading"
    >
      <Link
        to="/users"
        className="text-admin-accent hover:text-admin-accent-bright gap-admin-4 text-admin-meta inline-flex items-center font-mono no-underline before:content-['<-']"
      >
        Back to users
      </Link>
      <header className="border-admin-border flex items-center justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <span className="text-admin-ink text-admin-section bg-admin-surface-raised grid size-19 shrink-0 place-items-center rounded-full font-semibold">
          {initials(user.display_name)}
        </span>
        <div className="flex-1">
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Player dossier / {user.id}
          </p>
          <h1
            id="user-detail-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-detail-hero leading-none font-medium tracking-[-0.055em]"
          >
            {user.display_name}
          </h1>
          <p className="text-admin-muted text-admin-action m-0">
            @{user.username}
            {user.email ? ` / ${user.email}` : ''}
          </p>
        </div>
        <span
          className={`text-admin-caption px-admin-10 rounded-full border py-[0.38rem] font-mono uppercase ${user.suspension ? 'text-admin-danger border-admin-danger-border' : 'text-admin-success border-admin-success-border'}`}
        >
          {user.suspension ? 'Suspended' : 'Access active'}
        </span>
      </header>
      <dl className="border-admin-border-faint bg-admin-surface-translucent m-0 grid grid-cols-5 rounded-b-xl border border-t-0 max-[760px]:grid-cols-2 max-[480px]:grid-cols-1">
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
            Account version
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {user.version ?? 0}
          </dd>
        </div>
        <div className="border-admin-border-divider py-admin-15 border-r px-4 max-[480px]:border-r-0 max-[480px]:border-b">
          <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
            Providers
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.providers.length}
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
            Games retained
          </dt>
          <dd className="text-admin-ink-strong text-admin-field mt-1">
            {detail.games.length}
          </dd>
        </div>
      </dl>
      {message ? (
        <Notice variant={messageTone}>
          {message}{' '}
          {auditEventID ? (
            <Link to={`/audit-events/${auditEventID}`}>View audit event</Link>
          ) : null}
        </Notice>
      ) : null}
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(280px,350px)] items-start gap-6 max-[1100px]:grid-cols-1">
        <main className="min-w-0">
          <EvidenceSection
            title="Progression snapshot"
            eyebrow="Stats"
            items={Object.entries(detail.stats).map(([key, value]) => ({
              label: formatLabel(key),
              value,
            }))}
          />
          <RecordSection
            title="Rating history"
            eyebrow="Competitive history"
            records={detail.ratings}
          />
          <RecordSection
            title="Achievements"
            eyebrow="Progression grants"
            records={detail.achievements}
          />
          <RecordSection
            title="Skins"
            eyebrow="Cosmetic entitlements"
            records={detail.skins}
          />
          <RecordSection
            title="Game history"
            eyebrow="Recorded activity"
            records={detail.games}
          />
          {detail.room ? (
            <RecordSection
              title="Current room"
              eyebrow="Live context"
              records={[detail.room]}
            />
          ) : null}
        </main>
        <aside className="sticky top-6 max-[1100px]:static">
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel mt-6 grid gap-4 border p-5">
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
          </section>
        </aside>
      </div>
    </section>
  )
}

function EvidenceSection({
  title,
  eyebrow,
  items,
}: {
  title: string
  eyebrow: string
  items: Array<{ label: string; value: unknown }>
}) {
  return (
    <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel mt-6 border p-5">
      <SectionHeading
        eyebrow={eyebrow}
        title={title}
        id={`section-${title}`}
        meta={items.length}
      />
      {items.length ? (
        <dl className="border-admin-border-faint rounded-admin-rule mt-4 grid grid-cols-3 border max-[760px]:grid-cols-2 max-[480px]:grid-cols-1">
          {items.map((item) => (
            <div
              key={item.label}
              className="border-admin-border-faint p-admin-13 border-r border-b"
            >
              <dt className="text-admin-muted-subtle text-admin-caption font-mono uppercase">
                {item.label}
              </dt>
              <dd className="mt-admin-3 text-admin-ink-soft text-admin-field wrap-break-word">
                {displayValue(item.value)}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="text-admin-muted-subtle text-admin-field mt-4">
          No data retained.
        </p>
      )}
    </section>
  )
}
function RecordSection({
  title,
  eyebrow,
  records,
}: {
  title: string
  eyebrow: string
  records: Array<Record<string, unknown>>
}) {
  return (
    <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel mt-6 border p-5">
      <SectionHeading
        eyebrow={eyebrow}
        title={title}
        id={`section-${title}`}
        meta={records.length}
      />
      {records.length ? (
        <div className="gap-admin-8 mt-4 grid">
          {records.map((record, index) => (
            <dl
              key={index}
              className="border-admin-border-faint gap-admin-10 p-admin-13 m-0 grid grid-cols-[repeat(auto-fit,minmax(130px,1fr))] rounded-lg border"
            >
              {Object.entries(record).map(([key, value]) => (
                <div key={key}>
                  <dt className="text-admin-muted-subtle text-admin-caption font-mono uppercase">
                    {formatLabel(key)}
                  </dt>
                  <dd className="mt-admin-3 text-admin-ink-soft text-admin-field wrap-break-word">
                    {displayValue(value)}
                  </dd>
                </div>
              ))}
            </dl>
          ))}
        </div>
      ) : (
        <p className="text-admin-muted-subtle text-admin-field mt-4">
          No records retained.
        </p>
      )}
    </section>
  )
}
function displayValue(value: unknown) {
  if (value === null || value === undefined) return 'None'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
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
