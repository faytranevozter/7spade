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
import {
  FilterField,
  SectionHeading,
  SummaryItem,
} from '../components/InvestigationUI'
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
          className="text-admin-accent hover:text-admin-accent-bright inline-flex items-center gap-[0.35rem] font-mono text-[0.65rem] no-underline before:content-['<-']"
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
        className="text-admin-accent hover:text-admin-accent-bright inline-flex items-center gap-[0.35rem] font-mono text-[0.65rem] no-underline before:content-['<-']"
      >
        Back to users
      </Link>
      <header className="border-admin-ink/12 flex items-center justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <span className="text-admin-ink grid size-19 shrink-0 place-items-center rounded-full bg-[#235c36] text-[1.15rem] font-semibold">
          {initials(user.display_name)}
        </span>
        <div className="flex-1">
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Player dossier / {user.id}
          </p>
          <h1
            id="user-detail-heading"
            className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.2rem,4vw,3.8rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            {user.display_name}
          </h1>
          <p className="text-admin-muted m-0 text-[0.78rem]">
            @{user.username}
            {user.email ? ` / ${user.email}` : ''}
          </p>
        </div>
        <span
          className={`rounded-full border px-[0.7rem] py-[0.38rem] font-mono text-[0.58rem] uppercase ${user.suspension ? 'text-admin-danger border-[#c0392b]/55' : 'text-admin-success border-[#2d7a46]/55'}`}
        >
          {user.suspension ? 'Suspended' : 'Access active'}
        </span>
      </header>
      <dl className="border-admin-ink/10 bg-admin-surface/70 [&_.summary-healthy]:text-admin-success [&_dd]:text-admin-ink-strong [&_dt]:text-admin-muted-subtle [&>div]:border-admin-ink/9 m-0 grid grid-cols-5 rounded-b-xl border border-t-0 max-[760px]:grid-cols-2 max-[480px]:grid-cols-1 [&_dd]:mt-1 [&_dd]:text-[0.82rem] [&_dt]:font-mono [&_dt]:text-[0.57rem] [&_dt]:uppercase [&>div]:border-r [&>div]:px-4 [&>div]:py-[0.9rem] max-[480px]:[&>div]:border-r-0 max-[480px]:[&>div]:border-b [&>div:last-child]:border-0 max-[480px]:[&>div:last-child]:border-b-0 max-[760px]:[&>div:nth-child(2n)]:border-r-0">
        <SummaryItem
          label="Presence"
          value={user.online ? 'Online' : 'Offline'}
          tone={user.online ? 'healthy' : undefined}
        />
        <SummaryItem label="Account version" value={user.version ?? 0} />
        <SummaryItem label="Providers" value={detail.providers.length} />
        <SummaryItem label="Achievements" value={detail.achievements.length} />
        <SummaryItem label="Games retained" value={detail.games.length} />
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
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 grid gap-4 rounded-[14px] border p-5">
            <SectionHeading
              eyebrow="Operator controls"
              title="Moderation"
              id="moderation-heading"
            />
            {canModerate ? (
              <>
                <div className="border-admin-ink/9 grid gap-[0.7rem] border-t pt-4">
                  <h3 className="m-0 text-[0.76rem] text-[#d9d4c8]">
                    Display name
                  </h3>
                  <FilterField label="Replacement display name">
                    <input
                      value={displayName}
                      maxLength={50}
                      onChange={(event) => setDisplayName(event.target.value)}
                      className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-150 outline-none focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                    />
                  </FilterField>
                  <FilterField label="Moderation reason">
                    <input
                      value={displayNameReason}
                      onChange={(event) =>
                        setDisplayNameReason(event.target.value)
                      }
                      className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-150 outline-none focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                    />
                  </FilterField>
                  <button
                    disabled={
                      !displayName.trim() ||
                      !displayNameReason.trim() ||
                      displayName.trim() === user.display_name
                    }
                    onClick={() => void moderateDisplayName()}
                    className="text-admin-danger w-full cursor-pointer rounded-[7px] border border-[#c0392b]/55 bg-transparent px-[0.7rem] py-[0.55rem] font-mono text-[0.6rem]"
                  >
                    Replace display name
                  </button>
                </div>
                <div className="border-admin-ink/9 grid gap-[0.7rem] border-t pt-4">
                  <h3 className="m-0 text-[0.76rem] text-[#d9d4c8]">
                    Player access
                  </h3>
                  {user.suspension ? (
                    <div className="border-l-2 border-[#c0392b] bg-[#c0392b]/7 p-[0.7rem]">
                      <strong className="text-admin-danger block text-[0.7rem]">
                        {user.suspension.reason}
                      </strong>
                      <small className="text-admin-muted mt-1 block text-[0.6rem]">
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
                          className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-150 outline-none focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                        />
                      </FilterField>
                      <FilterField label="Expiry (optional)">
                        <input
                          type="datetime-local"
                          value={expiresAt}
                          onChange={(event) => setExpiresAt(event.target.value)}
                          className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-150 outline-none focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                        />
                      </FilterField>
                    </>
                  )}
                  <button
                    disabled={!user.suspension && !reason.trim()}
                    onClick={() => void updateSuspension()}
                    className={`w-full cursor-pointer rounded-[7px] border bg-transparent px-[0.7rem] py-[0.55rem] font-mono text-[0.6rem] ${user.suspension ? 'text-admin-success border-[#2d7a46]/55' : 'text-admin-danger border-[#c0392b]/55'}`}
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
    <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5">
      <SectionHeading
        eyebrow={eyebrow}
        title={title}
        id={`section-${title}`}
        meta={items.length}
      />
      {items.length ? (
        <dl className="border-admin-ink/8 [&_dt]:text-admin-muted-subtle [&>div]:border-admin-ink/8 mt-4 grid grid-cols-3 rounded-[9px] border max-[760px]:grid-cols-2 max-[480px]:grid-cols-1 [&_dd]:mt-[0.3rem] [&_dd]:text-[0.72rem] [&_dd]:wrap-break-word [&_dd]:text-[#d9d4c8] [&_dt]:font-mono [&_dt]:text-[0.56rem] [&_dt]:uppercase [&>div]:border-r [&>div]:border-b [&>div]:p-[0.8rem]">
          {items.map((item) => (
            <div key={item.label}>
              <dt>{item.label}</dt>
              <dd>{displayValue(item.value)}</dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="text-admin-muted-subtle mt-4 text-[0.7rem]">
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
    <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel mt-6 rounded-[14px] border p-5">
      <SectionHeading
        eyebrow={eyebrow}
        title={title}
        id={`section-${title}`}
        meta={records.length}
      />
      {records.length ? (
        <div className="[&_dl]:border-admin-ink/8 [&_dt]:text-admin-muted-subtle mt-4 grid gap-[0.6rem] [&_dd]:mt-[0.3rem] [&_dd]:text-[0.72rem] [&_dd]:wrap-break-word [&_dd]:text-[#d9d4c8] [&_dl]:m-0 [&_dl]:grid [&_dl]:grid-cols-[repeat(auto-fit,minmax(130px,1fr))] [&_dl]:gap-[0.7rem] [&_dl]:rounded-lg [&_dl]:border [&_dl]:p-[0.8rem] [&_dt]:font-mono [&_dt]:text-[0.56rem] [&_dt]:uppercase">
          {records.map((record, index) => (
            <dl key={index}>
              {Object.entries(record).map(([key, value]) => (
                <div key={key}>
                  <dt>{formatLabel(key)}</dt>
                  <dd>{displayValue(value)}</dd>
                </div>
              ))}
            </dl>
          ))}
        </div>
      ) : (
        <p className="text-admin-muted-subtle mt-4 text-[0.7rem]">
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
