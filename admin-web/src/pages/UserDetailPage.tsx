import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getUser, reinstateUser, suspendUser, updateUserDisplayName, type UserDetail } from '../api/users'
import { FilterField, SectionHeading, SummaryItem } from '../components/InvestigationUI'
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
    setDetail(next); setDisplayName((current) => current || next.user.display_name); setMessage('')
  }

  useEffect(() => {
    let cancelled = false
    if (!token || !id) return
    getUser(token, id).then((next) => { if (!cancelled) { setDetail(next); setDisplayName(next.user.display_name); setMessage('') } }).catch((error) => { if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to load user') })
    return () => { cancelled = true }
  }, [id, token])

  async function updateSuspension() {
    if (!detail || !token) return
    const suspending = !detail.user.suspension
    if (!window.confirm(`${suspending ? 'Suspend' : 'Reinstate'} ${detail.user.display_name} (${detail.user.id})? ${suspending ? 'This immediately revokes active Player credentials.' : 'This restores Player access.'}`)) return
    try {
      const response = suspending ? await suspendUser(token, detail.user.id, reason.trim(), expiresAt ? new Date(expiresAt).toISOString() : undefined) : await reinstateUser(token, detail.user.id)
      setAuditEventID(response.audit_event_id); setMessage(`${detail.user.display_name} ${suspending ? 'suspended' : 'reinstated'}.`); setMessageTone('success')
      try { await loadUser() } catch { setMessage(`${detail.user.display_name} ${suspending ? 'suspended' : 'reinstated'}, but the dossier could not be refreshed.`) }
    } catch (error) { setMessage(error instanceof Error ? error.message : 'Failed to update access'); setMessageTone('error') }
  }

  async function moderateDisplayName() {
    if (!detail || !token) return
    const nextName = displayName.trim()
    if (!window.confirm(`Replace ${detail.user.display_name} for ${detail.user.id} with ${nextName}?`)) return
    try {
      const response = await updateUserDisplayName(token, detail.user.id, nextName, displayNameReason.trim(), detail.user.version)
      setDetail({ ...detail, user: response.user }); setDisplayName(response.user.display_name); setDisplayNameReason(''); setAuditEventID(response.audit_event_id); setMessage(`Display name updated to ${response.user.display_name}.`); setMessageTone('success')
    } catch (error) {
      if (error instanceof ApiError && error.status === 409 && error.message === 'User changed since it was loaded') {
        try { const current = await getUser(token, detail.user.id); setDetail(current); setMessage(`This player changed since you loaded it. Current name: ${current.user.display_name}. Review your attempted replacement and submit again.`) } catch { setMessage('This player changed since you loaded it, but the latest version could not be loaded. Reload before retrying.') }
      } else setMessage(error instanceof Error ? error.message : 'Failed to update display name')
      setMessageTone('error')
    }
  }

  if (!detail || detail.user.id !== id) return <section className="user-detail-page"><Link to="/users" className="back-link">Back to users</Link><Notice variant="info" role="alert">{detail ? 'Loading user...' : message}</Notice></section>
  const { user } = detail
  return <section className="user-detail-page" aria-labelledby="user-detail-heading">
    <Link to="/users" className="back-link">Back to users</Link>
    <header className="user-detail-hero"><span className="user-hero-avatar">{initials(user.display_name)}</span><div><p className="eyebrow">Player dossier / {user.id}</p><h1 id="user-detail-heading">{user.display_name}</h1><p>@{user.username}{user.email ? ` / ${user.email}` : ''}</p></div><span className={user.suspension ? 'user-state suspended' : 'user-state active'}>{user.suspension ? 'Suspended' : 'Access active'}</span></header>
    <dl className="user-summary-strip"><SummaryItem label="Presence" value={user.online ? 'Online' : 'Offline'} tone={user.online ? 'healthy' : undefined} /><SummaryItem label="Account version" value={user.version ?? 0} /><SummaryItem label="Providers" value={detail.providers.length} /><SummaryItem label="Achievements" value={detail.achievements.length} /><SummaryItem label="Games retained" value={detail.games.length} /></dl>
    {message ? <Notice variant={messageTone}>{message} {auditEventID ? <Link to={`/audit-events/${auditEventID}`}>View audit event</Link> : null}</Notice> : null}
    <div className="user-dossier-layout"><main className="user-evidence-column">
      <EvidenceSection title="Progression snapshot" eyebrow="Stats" items={Object.entries(detail.stats).map(([key, value]) => ({ label: formatLabel(key), value }))} />
      <RecordSection title="Rating history" eyebrow="Competitive history" records={detail.ratings} />
      <RecordSection title="Achievements" eyebrow="Progression grants" records={detail.achievements} />
      <RecordSection title="Skins" eyebrow="Cosmetic entitlements" records={detail.skins} />
      <RecordSection title="Game history" eyebrow="Recorded activity" records={detail.games} />
      {detail.room ? <RecordSection title="Current room" eyebrow="Live context" records={[detail.room]} /> : null}
    </main><aside className="moderation-rail"><section className="moderation-panel"><SectionHeading eyebrow="Operator controls" title="Moderation" id="moderation-heading" />
      {canModerate ? <><div className="moderation-block"><h3>Display name</h3><FilterField label="Replacement display name"><input value={displayName} maxLength={50} onChange={(event) => setDisplayName(event.target.value)} className="game-input" /></FilterField><FilterField label="Moderation reason"><input value={displayNameReason} onChange={(event) => setDisplayNameReason(event.target.value)} className="game-input" /></FilterField><button disabled={!displayName.trim() || !displayNameReason.trim() || displayName.trim() === user.display_name} onClick={() => void moderateDisplayName()} className="danger-outline-action">Replace display name</button></div><div className="moderation-block"><h3>Player access</h3>{user.suspension ? <div className="suspension-record"><strong>{user.suspension.reason}</strong><small>{user.suspension.expires_at ? `Until ${formatDateTime(user.suspension.expires_at)}` : 'Indefinite suspension'}</small></div> : <><FilterField label="Reason"><input value={reason} onChange={(event) => setReason(event.target.value)} className="game-input" /></FilterField><FilterField label="Expiry (optional)"><input type="datetime-local" value={expiresAt} onChange={(event) => setExpiresAt(event.target.value)} className="game-input" /></FilterField></>}<button disabled={!user.suspension && !reason.trim()} onClick={() => void updateSuspension()} className={user.suspension ? 'success-outline-action' : 'danger-outline-action'}>{user.suspension ? 'Reinstate player' : 'Suspend player'}</button></div></> : <ReadOnlyNotice>You can inspect this dossier but cannot moderate the player.</ReadOnlyNotice>}
    </section></aside></div>
  </section>
}

function EvidenceSection({ title, eyebrow, items }: { title: string; eyebrow: string; items: Array<{ label: string; value: unknown }> }) { return <section className="dossier-panel"><SectionHeading eyebrow={eyebrow} title={title} id={`section-${title}`} meta={items.length} />{items.length ? <dl className="evidence-grid">{items.map((item) => <div key={item.label}><dt>{item.label}</dt><dd>{displayValue(item.value)}</dd></div>)}</dl> : <p className="evidence-empty">No data retained.</p>}</section> }
function RecordSection({ title, eyebrow, records }: { title: string; eyebrow: string; records: Array<Record<string, unknown>> }) { return <section className="dossier-panel"><SectionHeading eyebrow={eyebrow} title={title} id={`section-${title}`} meta={records.length} />{records.length ? <div className="record-list">{records.map((record, index) => <dl key={index}>{Object.entries(record).map(([key, value]) => <div key={key}><dt>{formatLabel(key)}</dt><dd>{displayValue(value)}</dd></div>)}</dl>)}</div> : <p className="evidence-empty">No records retained.</p>}</section> }
function displayValue(value: unknown) { if (value === null || value === undefined) return 'None'; if (typeof value === 'object') return JSON.stringify(value); return String(value) }
function initials(name: string) { return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || '?' }
