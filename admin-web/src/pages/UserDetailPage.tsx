import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getUser, reinstateUser, suspendUser, updateUserDisplayName, type UserDetail } from '../api/users'
import { useAuth } from '../hooks/useAuth'

export function UserDetailPage() {
  const { id = '' } = useParams()
  const { admin, token } = useAuth()
  const [detail, setDetail] = useState<UserDetail | null>(null)
  const [message, setMessage] = useState('Loading user...')
  const [auditEventID, setAuditEventID] = useState('')
  const [reason, setReason] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [displayNameReason, setDisplayNameReason] = useState('')

  async function loadUser() {
    if (!token || !id) return
    setMessage('Loading user...')
    try {
      const nextDetail = await getUser(token, id)
      setDetail(nextDetail)
      setDisplayName((current) => current || nextDetail.user.display_name)
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to load user')
    }
  }

  useEffect(() => { void loadUser() }, [id, token])

  async function updateSuspension() {
    if (!detail || !token) return
    const action = detail.user.suspension ? 'reinstate' : 'suspend'
    const impact = action === 'suspend' ? 'This immediately revokes active Player credentials.' : 'This restores Player access.'
    if (!window.confirm(`${action === 'suspend' ? 'Suspend' : 'Reinstate'} ${detail.user.display_name} (${detail.user.id})? ${impact}`)) return
    try {
      const response = detail.user.suspension
        ? await reinstateUser(token, detail.user.id)
        : await suspendUser(token, detail.user.id, reason.trim(), expiresAt ? new Date(expiresAt).toISOString() : undefined)
      setAuditEventID(response.audit_event_id)
      await loadUser()
      setMessage(`${detail.user.display_name} ${action === 'suspend' ? 'suspended' : 'reinstated'}.`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `Failed to ${action} user`)
    }
  }

  async function moderateDisplayName() {
    if (!detail || !token) return
    const nextName = displayName.trim()
    if (!window.confirm(`Replace ${detail.user.display_name} for ${detail.user.id} with ${nextName}?`)) return
    try {
      const response = await updateUserDisplayName(token, detail.user.id, nextName, displayNameReason.trim(), detail.user.version)
      setDetail({ ...detail, user: response.user })
      setDisplayName(response.user.display_name)
      setDisplayNameReason('')
      setAuditEventID(response.audit_event_id)
      setMessage(`Display name updated to ${response.user.display_name}.`)
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        if (error.message !== 'User changed since it was loaded') {
          setMessage(error.message)
          return
        }
        try {
          const current = await getUser(token, detail.user.id)
          setDetail(current)
          setMessage(`This player changed since you loaded it. Current name: ${current.user.display_name}. Review your attempted replacement and submit again.`)
        } catch {
          setMessage('This player changed since you loaded it, but the latest version could not be loaded. Reload the page before retrying.')
        }
        return
      }
      setMessage(error instanceof Error ? error.message : 'Failed to update display name')
    }
  }

  return <section aria-labelledby="user-detail-heading">
    <Link to="/users" className="text-sm text-[#4dd0b5] underline">Back to users</Link>
    {message ? <p role="alert" className="mt-4 text-[#ffaaa4]">{message} {auditEventID ? <Link to={`/audit-events/${auditEventID}`} className="text-[#4dd0b5] underline">View audit event</Link> : null}</p> : null}
    {detail ? <UserDetailPanel detail={detail} canModerate={admin?.permissions.includes('users.moderate') ?? false} reason={reason} expiresAt={expiresAt} displayName={displayName} displayNameReason={displayNameReason} onReasonChange={setReason} onExpiresAtChange={setExpiresAt} onDisplayNameChange={setDisplayName} onDisplayNameReasonChange={setDisplayNameReason} onUpdateSuspension={updateSuspension} onUpdateDisplayName={moderateDisplayName} /> : null}
  </section>
}

function UserDetailPanel({ detail, canModerate, reason, expiresAt, displayName, displayNameReason, onReasonChange, onExpiresAtChange, onDisplayNameChange, onDisplayNameReasonChange, onUpdateSuspension, onUpdateDisplayName }: { detail: UserDetail; canModerate: boolean; reason: string; expiresAt: string; displayName: string; displayNameReason: string; onReasonChange: (value: string) => void; onExpiresAtChange: (value: string) => void; onDisplayNameChange: (value: string) => void; onDisplayNameReasonChange: (value: string) => void; onUpdateSuspension: () => Promise<void>; onUpdateDisplayName: () => Promise<void> }) {
  const { user } = detail
  return <section className="mt-5 border border-[#28323d] bg-[#10161d] p-5" aria-labelledby="user-detail-heading">
    <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">PLAYER DETAIL</p>
    <h2 id="user-detail-heading" className="mt-2 text-xl font-bold text-white">{user.display_name}</h2>
    <p className="text-[#8493a5]">@{user.username} · {user.id}</p>
    {user.email ? <p className="text-[#aeb8c4]">{user.email}</p> : null}
    {canModerate ? <section className="mt-5 border-t border-[#28323d] pt-4">
      <h3 className="text-sm font-bold text-white">Display name moderation</h3>
      <div className="grid max-w-md gap-2">
        <label className="grid gap-1 text-sm text-[#aeb8c4]">Replacement display name<input value={displayName} maxLength={50} onChange={(event) => onDisplayNameChange(event.target.value)} className="border border-[#394552] bg-[#0c1117] px-3 py-2 text-white" /></label>
        <label className="grid gap-1 text-sm text-[#aeb8c4]">Moderation reason<input value={displayNameReason} onChange={(event) => onDisplayNameReasonChange(event.target.value)} className="border border-[#394552] bg-[#0c1117] px-3 py-2 text-white" /></label>
        <button type="button" disabled={!displayName.trim() || !displayNameReason.trim() || displayName.trim() === user.display_name} onClick={() => void onUpdateDisplayName()} className="border border-[#ff786f] px-3 py-2 text-sm text-[#ffaaa4] disabled:opacity-50">Replace display name</button>
      </div>
    </section> : null}
    <section className="mt-5 border-t border-[#28323d] pt-4">
      <h3 className="text-sm font-bold text-white">Access moderation</h3>
      {user.suspension ? <p className="text-sm text-[#ffaaa4]">Suspended: {user.suspension.reason}{user.suspension.expires_at ? ` until ${user.suspension.expires_at}` : ' indefinitely'}</p> : <p className="text-sm text-[#4dd0b5]">Access active</p>}
      {canModerate ? user.suspension ? <button type="button" onClick={() => void onUpdateSuspension()} className="border border-[#4dd0b5] px-3 py-2 text-sm text-[#4dd0b5]">Reinstate player</button> : <div className="grid max-w-md gap-2">
        <label className="grid gap-1 text-sm text-[#aeb8c4]">Reason<input value={reason} onChange={(event) => onReasonChange(event.target.value)} required className="border border-[#394552] bg-[#0c1117] px-3 py-2 text-white" /></label>
        <label className="grid gap-1 text-sm text-[#aeb8c4]">Expiry (optional)<input type="datetime-local" value={expiresAt} onChange={(event) => onExpiresAtChange(event.target.value)} className="border border-[#394552] bg-[#0c1117] px-3 py-2 text-white" /></label>
        <button type="button" disabled={!reason.trim()} onClick={() => void onUpdateSuspension()} className="border border-[#ff786f] px-3 py-2 text-sm text-[#ffaaa4] disabled:opacity-50">Suspend player</button>
      </div> : null}
    </section>
    <DetailList title="Linked providers" items={detail.providers} />
    <DetailList title="Stats" items={Object.entries(detail.stats).map(([key, value]) => `${key.replaceAll('_', ' ')}: ${String(value)}`)} />
    <DetailList title="Rating history" items={detail.ratings.map((rating) => JSON.stringify(rating))} />
    <DetailList title="Achievements" items={detail.achievements.map((achievement) => JSON.stringify(achievement))} />
    <DetailList title="Skins" items={detail.skins.map((skin) => JSON.stringify(skin))} />
    <DetailList title="Game history" items={detail.games.map((game) => JSON.stringify(game))} />
    {detail.room ? <DetailList title="Current room" items={[JSON.stringify(detail.room)]} /> : null}
  </section>
}

function DetailList({ title, items }: { title: string; items: string[] }) {
  return <section className="mt-5">
    <h3 className="text-sm font-bold text-white">{title}</h3>
    {items.length ? <ul className="mt-2 grid gap-1 pl-5 text-sm text-[#aeb8c4]">{items.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul> : <p className="text-sm text-[#8493a5]">None</p>}
  </section>
}
