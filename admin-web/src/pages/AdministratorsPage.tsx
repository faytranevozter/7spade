import { useEffect, useState } from 'react'
import { getAdmins, getRoles, inviteAdmin, setAdminRoles, setAdminStatus, type Admin, type Role } from '../api/auth'
import { CredentialNotice, LoadingState, Notice, ReadOnlyNotice } from '../components/Feedback'
import { EmptyState, FilterField, SectionHeading } from '../components/InvestigationUI'
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function AdministratorsPage() {
  const { admin, token } = useAuth()
  const [admins, setAdmins] = useState<Admin[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRoleID, setInviteRoleID] = useState('')
  const [inviteToken, setInviteToken] = useState('')
  const canManage = admin?.permissions.includes('admins.manage') ?? false

  useEffect(() => {
    if (!token || !admin?.permissions.includes('admins.read')) return
    let cancelled = false
    Promise.all([getAdmins(token), getRoles(token)]).then(([nextAdmins, nextRoles]) => {
      if (cancelled) return
      setAdmins(nextAdmins ?? [])
      setRoles(nextRoles ?? [])
      setInviteRoleID((current) => current || nextRoles[0]?.id || '')
      setMessage('')
    }).catch((error: unknown) => {
      if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to load administrators')
    }).finally(() => {
      if (!cancelled) setLoading(false)
    })
    return () => { cancelled = true }
  }, [admin, token])

  async function handleInvite(event: React.FormEvent) {
    event.preventDefault()
    if (!token || !inviteEmail.trim() || !inviteRoleID) return
    try {
      const response = await inviteAdmin(token, inviteEmail.trim(), inviteRoleID)
      setInviteToken(response.token)
      setMessage(`Invitation created for ${response.invitation.email}`)
      setInviteEmail('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to invite administrator')
    }
  }

  async function toggleStatus(target: Admin) {
    if (!token) return
    const nextStatus = target.status === 'active' ? 'disabled' : 'active'
    const action = nextStatus === 'disabled' ? 'disable' : 'activate'
    if (!window.confirm(`${formatLabel(action)} ${target.display_name} (${target.email})? Their active authorization state will be revoked or refreshed.`)) return
    try {
      await setAdminStatus(token, target.id, nextStatus)
      setAdmins((current) => current.map((item) => item.id === target.id ? { ...item, status: nextStatus } : item))
      setMessage(`Administrator ${target.email} is now ${nextStatus}`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update administrator status')
    }
  }

  async function changeRole(target: Admin, roleID: string) {
    if (!token) return
    try {
      await setAdminRoles(token, target.id, [roleID])
      const role = roles.find((item) => item.id === roleID)
      setAdmins((current) => current.map((item) => item.id === target.id ? { ...item, roles: role ? [role] : [] } : item))
      setMessage('Administrator role updated')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to update administrator role')
    }
  }

  if (!admin) return null
  const normalizedQuery = query.trim().toLowerCase()
  const visibleAdmins = admins.filter((item) => {
    const matchesQuery = !normalizedQuery || `${item.display_name} ${item.email} ${item.id}`.toLowerCase().includes(normalizedQuery)
    return matchesQuery && (!status || item.status === status)
  })
  const activeCount = admins.filter((item) => item.status === 'active').length
  const mfaCount = admins.filter((item) => item.mfa_enrolled).length

  return <section className="access-page" aria-labelledby="admins-heading">
    <header className="access-header">
      <div><p className="eyebrow">Access control / Identities</p><h1 id="admins-heading">Administrators</h1><p>Invite operators, review their access posture, and manage identity lifecycle independently from role policy.</p></div>
      <div className="access-header-stats"><div><strong>{admins.length}</strong><span>Total identities</span></div><div><strong>{activeCount}</strong><span>Active</span></div><div><strong>{mfaCount}</strong><span>MFA enrolled</span></div></div>
    </header>

    {message ? <Notice variant="success">{message}</Notice> : null}
    {inviteToken ? <CredentialNotice eyebrow="One-time invitation credential" credential={`Token: ${inviteToken}`} description="Store this securely. It is displayed only for this response." onDismiss={() => setInviteToken('')} /> : null}

    <div className="access-layout">
      <main className="administrator-directory">
        <section className="access-panel">
          <SectionHeading eyebrow="Identity directory" title="Administrator list" id="administrator-list-heading" meta={`${visibleAdmins.length} shown`} />
          <div className="admin-list-toolbar">
            <FilterField label="Search administrators"><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, email, or administrator ID" className="game-input" /></FilterField>
            <FilterField label="Account status"><select value={status} onChange={(event) => setStatus(event.target.value)} className="game-input"><option value="">Any status</option><option value="active">Active</option><option value="disabled">Disabled</option><option value="invited">Invited</option></select></FilterField>
          </div>
          {loading ? <LoadingState>Loading administrator directory...</LoadingState> : null}
          {!loading && visibleAdmins.length === 0 ? <EmptyState mark="A" title="No administrators found" description="Try another search or clear the status filter." /> : null}
          <div className="administrator-list">{visibleAdmins.map((item) => <AdministratorRow key={item.id} item={item} currentID={admin.id} roles={roles} canManage={canManage} onRoleChange={changeRole} onToggleStatus={toggleStatus} />)}</div>
        </section>
      </main>

      <aside className="invite-rail">
        <section className="access-panel invite-panel">
          <p className="eyebrow">Provision access</p><h2>Invite administrator</h2><p>New administrators receive one initial role. Permission policy is managed separately under Roles & permissions.</p>
          {canManage ? <form onSubmit={handleInvite} className="invite-form">
            <FilterField label="Invite email"><input id="invite-email" aria-label="Invite Email" type="email" required value={inviteEmail} onChange={(event) => setInviteEmail(event.target.value)} placeholder="operator@example.com" className="game-input" /></FilterField>
            <FilterField label="Initial role"><select id="invite-role" aria-label="Role" value={inviteRoleID} onChange={(event) => setInviteRoleID(event.target.value)} className="game-input">{roles.map((role) => <option key={role.id} value={role.id}>{formatLabel(role.name)}</option>)}</select></FilterField>
            <button type="submit" disabled={!inviteEmail.trim() || !inviteRoleID} className="primary-action">Send Invite</button>
          </form> : <ReadOnlyNotice>You can inspect administrators but cannot provision or change identities.</ReadOnlyNotice>}
        </section>
      </aside>
    </div>
  </section>
}

function AdministratorRow({ item, currentID, roles, canManage, onRoleChange, onToggleStatus }: { item: Admin; currentID: string; roles: Role[]; canManage: boolean; onRoleChange: (admin: Admin, roleID: string) => Promise<void>; onToggleStatus: (admin: Admin) => Promise<void> }) {
  const isCurrent = item.id === currentID
  return <article className="administrator-row">
    <span className="admin-avatar">{initials(item.display_name)}</span>
    <div className="admin-identity"><div><strong>{item.display_name}</strong>{isCurrent ? <span className="current-admin-badge">You</span> : null}<span className={`admin-status admin-status-${item.status}`}>{formatLabel(item.status)}</span></div><p>{item.email}</p><small>{item.created_at ? `Created ${formatDateTime(item.created_at)}` : item.id}</small></div>
    <div className="admin-security"><span className={item.mfa_enrolled ? 'security-good' : 'security-warning'}>{item.mfa_enrolled ? 'MFA enrolled' : 'MFA not enrolled'}</span><small>{item.roles?.map((role) => formatLabel(role.name)).join(', ') || 'No role assigned'}</small></div>
    {canManage && !isCurrent ? <div className="admin-row-actions"><select aria-label={`Role for ${item.display_name}`} value={item.roles?.[0]?.id ?? ''} onChange={(event) => void onRoleChange(item, event.target.value)} className="game-input">{roles.map((role) => <option key={role.id} value={role.id}>{formatLabel(role.name)}</option>)}</select><button type="button" onClick={() => void onToggleStatus(item)} className={item.status === 'active' ? 'danger-outline-action' : 'success-outline-action'}>{item.status === 'active' ? 'Disable' : 'Activate'}</button></div> : <div className="admin-row-actions"><span className="locked-identity">{isCurrent ? 'Current identity' : 'View only'}</span></div>}
  </article>
}

function initials(name: string) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || '?'
}
