import { useEffect, useState } from 'react'
import {
  getAdmins,
  getInvitations,
  getRoles,
  inviteAdmin,
  reissueInvitation,
  revokeInvitation,
  setAdminRoles,
  setAdminStatus,
  type Admin,
  type Role,
  type Invitation,
} from '../api/auth'
import {
  CredentialNotice,
  LoadingState,
  Notice,
  ReadOnlyNotice,
} from '../components/Feedback'
import {
  EmptyState,
  FilterField,
  SectionHeading,
} from '../components/InvestigationUI'
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function AdministratorsPage() {
  const { admin, token } = useAuth()
  const [admins, setAdmins] = useState<Admin[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState('')
  const [messageTone, setMessageTone] = useState<'success' | 'error'>('error')
  const [loading, setLoading] = useState(true)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRoleID, setInviteRoleID] = useState('')
  const [inviteToken, setInviteToken] = useState('')
  const canManage = admin?.permissions.includes('admins.manage') ?? false

  useEffect(() => {
    if (!token || !admin?.permissions.includes('admins.read')) return
    let cancelled = false
    Promise.all([getAdmins(token), getRoles(token)])
      .then(([nextAdmins, nextRoles]) => {
        if (cancelled) return
        setAdmins(nextAdmins ?? [])
        setRoles(nextRoles ?? [])
        setInviteRoleID((current) => current || nextRoles[0]?.id || '')
        setMessage('')
      })
      .catch((error: unknown) => {
        if (!cancelled) setMessageTone('error')
        if (!cancelled)
          setMessage(
            error instanceof Error
              ? error.message
              : 'Failed to load administrators',
          )
      })
    getInvitations(token)
      .then((nextInvitations) => {
        if (!cancelled) setInvitations(nextInvitations ?? [])
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [admin, token])

  async function handleInvite(event: React.FormEvent) {
    event.preventDefault()
    if (!token || !inviteEmail.trim() || !inviteRoleID) return
    try {
      const response = await inviteAdmin(
        token,
        inviteEmail.trim(),
        inviteRoleID,
      )
      setInviteToken(response.token)
      setInvitations((current) => [response.invitation, ...current])
      setMessage(`Invitation created for ${response.invitation.email}`)
      setMessageTone('success')
      setInviteEmail('')
    } catch (error) {
      setMessageTone('error')
      setMessage(
        error instanceof Error
          ? error.message
          : 'Failed to invite administrator',
      )
    }
  }

  function invitationLink(token: string) {
    return `${window.location.origin}${window.location.pathname}#/accept-invitation?token=${encodeURIComponent(token)}`
  }

  async function copyInviteLink(rawToken: string) {
    try {
      await navigator.clipboard.writeText(invitationLink(rawToken))
      setMessage('Invitation link copied')
      setMessageTone('success')
    } catch {
      setMessage('Could not copy the invitation link')
      setMessageTone('error')
    }
  }

  async function revoke(invitation: Invitation) {
    if (!token || !window.confirm(`Revoke the invitation for ${invitation.email}?`)) return
    try {
      await revokeInvitation(token, invitation.id)
      setInvitations((current) => current.map((item) => item.id === invitation.id ? { ...item, revoked_at: new Date().toISOString() } : item))
      setMessage(`Invitation for ${invitation.email} revoked`)
      setMessageTone('success')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to revoke invitation')
      setMessageTone('error')
    }
  }

  async function reissue(invitation: Invitation) {
    if (!token) return
    try {
      const response = await reissueInvitation(token, invitation.id)
      setInvitations((current) => current.map((item) => item.id === invitation.id ? response.invitation : item))
      setInviteToken(response.token)
      setMessage(`Invitation for ${invitation.email} reissued`)
      setMessageTone('success')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to reissue invitation')
      setMessageTone('error')
    }
  }

  async function toggleStatus(target: Admin) {
    if (!token) return
    const nextStatus = target.status === 'active' ? 'disabled' : 'active'
    const action = nextStatus === 'disabled' ? 'disable' : 'activate'
    if (
      !window.confirm(
        `${formatLabel(action)} ${target.display_name} (${target.email})? Their active authorization state will be revoked or refreshed.`,
      )
    )
      return
    try {
      await setAdminStatus(token, target.id, nextStatus)
      setAdmins((current) =>
        current.map((item) =>
          item.id === target.id ? { ...item, status: nextStatus } : item,
        ),
      )
      setMessage(`Administrator ${target.email} is now ${nextStatus}`)
      setMessageTone('success')
    } catch (error) {
      setMessageTone('error')
      setMessage(
        error instanceof Error
          ? error.message
          : 'Failed to update administrator status',
      )
    }
  }

  async function changeRole(target: Admin, roleID: string) {
    if (!token) return
    try {
      await setAdminRoles(token, target.id, [roleID])
      const role = roles.find((item) => item.id === roleID)
      setAdmins((current) =>
        current.map((item) =>
          item.id === target.id ? { ...item, roles: role ? [role] : [] } : item,
        ),
      )
      setMessage('Administrator role updated')
      setMessageTone('success')
    } catch (error) {
      setMessageTone('error')
      setMessage(
        error instanceof Error
          ? error.message
          : 'Failed to update administrator role',
      )
    }
  }

  if (!admin) return null
  const normalizedQuery = query.trim().toLowerCase()
  const visibleAdmins = admins.filter((item) => {
    const matchesQuery =
      !normalizedQuery ||
      `${item.display_name} ${item.email} ${item.id}`
        .toLowerCase()
        .includes(normalizedQuery)
    return matchesQuery && (!status || item.status === status)
  })
  const activeCount = admins.filter((item) => item.status === 'active').length
  const mfaCount = admins.filter((item) => item.mfa_enrolled).length

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="admins-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[720px]:flex-col max-[720px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Access control / Identities
          </p>
          <h1
            id="admins-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-investigation-hero mr-0 ml-0 font-medium"
          >
            Administrators
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Invite operators, review their access posture, and manage identity
            lifecycle independently from role policy.
          </p>
        </div>
        <div className="border-admin-ink/11 bg-admin-surface/80 grid min-w-87.5 grid-cols-3 overflow-hidden rounded-xl border max-[720px]:min-w-0 max-[480px]:grid-cols-1">
          <div className="border-admin-ink/9 p-admin-14 border-r last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {admins.length}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              Total identities
            </span>
          </div>
          <div className="border-admin-ink/9 p-admin-14 border-r last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {activeCount}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              Active
            </span>
          </div>
          <div className="border-admin-ink/9 p-admin-14 border-r last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {mfaCount}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              MFA enrolled
            </span>
          </div>
        </div>
      </header>

      {message ? <Notice variant={messageTone}>{message}</Notice> : null}
      {inviteToken ? (
        <div>
          <CredentialNotice
            eyebrow="One-time invitation credential"
            credential={`Token: ${inviteToken}`}
            description="Share the invitation link securely. It is displayed only for this response."
            onDismiss={() => setInviteToken('')}
          />
          <button type="button" onClick={() => void copyInviteLink(inviteToken)} className="border-admin-accent text-admin-accent mt-3 w-fit rounded-md border px-3 py-2 font-semibold">
            Copy invitation link
          </button>
        </div>
      ) : null}

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,350px)] items-start gap-6 max-[900px]:grid-cols-1">
        <main>
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel border p-5">
            <SectionHeading
              eyebrow="Identity directory"
              title="Administrator list"
              id="administrator-list-heading"
              meta={`${visibleAdmins.length} shown`}
            />
            <div className="my-admin-16 gap-admin-13 grid grid-cols-[minmax(0,1fr)_180px] max-[720px]:grid-cols-1">
              <FilterField label="Search administrators">
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Name, email, or administrator ID"
                  className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                />
              </FilterField>
              <FilterField label="Account status">
                <select
                  value={status}
                  onChange={(event) => setStatus(event.target.value)}
                  className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                >
                  <option value="">Any status</option>
                  <option value="active">Active</option>
                  <option value="disabled">Disabled</option>
                </select>
              </FilterField>
            </div>
            {loading ? (
              <LoadingState>Loading administrator directory...</LoadingState>
            ) : null}
            {!loading && visibleAdmins.length === 0 ? (
              <EmptyState
                mark="A"
                title="No administrators found"
                description="Try another search or clear the status filter."
              />
            ) : null}
            <div className="gap-admin-7 grid">
              {visibleAdmins.map((item) => (
                <AdministratorRow
                  key={item.id}
                  item={item}
                  currentID={admin.id}
                  roles={roles}
                  canManage={canManage}
                  onRoleChange={changeRole}
                  onToggleStatus={toggleStatus}
                />
              ))}
            </div>
          </section>
        </main>

        <aside className="sticky top-6 max-[900px]:static">
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel border p-5">
            <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
              Provision access
            </p>
            <h2 className="text-admin-ink-strong my-admin-5 text-admin-heading">
              Invite administrator
            </h2>
            <p className="text-admin-muted text-admin-body m-0 leading-[1.55]">
              New administrators receive one initial role. Permission policy is
              managed separately under Roles & permissions.
            </p>
            {canManage ? (
              <form
                onSubmit={handleInvite}
                className="border-admin-ink/9 mt-admin-17 gap-admin-15 pt-admin-16 grid border-t"
              >
                <FilterField label="Invite email">
                  <input
                    id="invite-email"
                    aria-label="Invite Email"
                    type="email"
                    required
                    value={inviteEmail}
                    onChange={(event) => setInviteEmail(event.target.value)}
                    placeholder="operator@example.com"
                    className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </FilterField>
                <FilterField label="Initial role">
                  <select
                    id="invite-role"
                    aria-label="Role"
                    value={inviteRoleID}
                    onChange={(event) => setInviteRoleID(event.target.value)}
                    className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  >
                    {roles.map((role) => (
                      <option key={role.id} value={role.id}>
                        {formatLabel(role.name)}
                      </option>
                    ))}
                  </select>
                </FilterField>
                <button
                  type="submit"
                  disabled={!inviteEmail.trim() || !inviteRoleID}
                  className="border-admin-accent bg-admin-accent rounded-admin-input text-admin-button-ink p-admin-10 text-admin-field cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-35"
                >
                  Send Invite
                </button>
              </form>
            ) : (
              <ReadOnlyNotice>
                You can inspect administrators but cannot provision or change
                identities.
              </ReadOnlyNotice>
            )}
            <div className="border-admin-ink/9 mt-6 grid gap-3 border-t pt-5">
              <h3 className="text-admin-ink-strong text-admin-field font-semibold">Invitation history</h3>
              {invitations.length === 0 ? <p className="text-admin-muted text-sm">No invitations yet.</p> : null}
              {invitations.map((invitation) => {
                const active = Boolean(invitation.expires_at) && !invitation.accepted_at && !invitation.revoked_at && new Date(invitation.expires_at) > new Date()
                const state = invitation.accepted_at ? 'Accepted' : invitation.revoked_at ? 'Revoked' : active ? 'Pending' : 'Expired'
                return (
                  <article key={invitation.id} className="border-admin-ink/10 grid gap-2 rounded-md border p-3">
                    <strong className="text-admin-ink text-sm">{invitation.email}</strong>
                    <span className="text-admin-muted-subtle text-xs">{formatLabel(invitation.role_name ?? 'Administrator')} · {state}{invitation.expires_at ? ` · expires ${formatDateTime(invitation.expires_at)}` : ''}</span>
                    {canManage && active ? (
                      <div className="flex gap-2">
                        <button type="button" onClick={() => void reissue(invitation)} className="border-admin-accent text-admin-accent rounded-md border px-2 py-1 text-xs">Reissue</button>
                        <button type="button" onClick={() => void revoke(invitation)} className="border-admin-danger-border text-admin-danger rounded-md border px-2 py-1 text-xs">Revoke</button>
                      </div>
                    ) : null}
                  </article>
                )
              })}
            </div>
          </section>
        </aside>
      </div>
    </section>
  )
}

function AdministratorRow({
  item,
  currentID,
  roles,
  canManage,
  onRoleChange,
  onToggleStatus,
}: {
  item: Admin
  currentID: string
  roles: Role[]
  canManage: boolean
  onRoleChange: (admin: Admin, roleID: string) => Promise<void>
  onToggleStatus: (admin: Admin) => Promise<void>
}) {
  const isCurrent = item.id === currentID
  return (
    <article className="border-admin-ink/9 hover:border-admin-accent/30 gap-admin-14 rounded-admin-rule p-admin-13 grid grid-cols-[44px_minmax(200px,1fr)_minmax(130px,0.55fr)_minmax(190px,auto)] items-center border transition-[border-color,background] duration-120 hover:bg-white/2 max-[1150px]:grid-cols-[44px_minmax(190px,1fr)_minmax(180px,auto)] max-[720px]:grid-cols-[40px_minmax(0,1fr)] max-[480px]:grid-cols-1">
      <span className="text-admin-ink bg-admin-success-bg text-admin-field grid size-10 place-items-center rounded-full font-semibold">
        {initials(item.display_name)}
      </span>
      <div>
        <div className="gap-admin-5 flex flex-wrap items-center">
          <strong className="text-admin-ink text-admin-value">
            {item.display_name}
          </strong>
          {isCurrent ? (
            <span className="border-admin-accent/40 text-admin-warning px-admin-badge-x py-admin-badge-y text-admin-xs rounded-full border font-mono uppercase">
              You
            </span>
          ) : null}
          <span
            className={`px-admin-badge-x py-admin-badge-y text-admin-xs rounded-full border font-mono uppercase ${item.status === 'active' ? 'text-admin-success border-admin-success-border' : item.status === 'disabled' ? 'text-admin-danger border-admin-danger-border' : 'border-admin-ink/15 text-admin-warning'}`}
          >
            {formatLabel(item.status)}
          </span>
        </div>
        <p className="text-admin-muted text-admin-note my-admin-chip-y">
          {item.email}
        </p>
        <small className="text-admin-muted-subtle text-admin-xs block max-w-75 overflow-hidden font-mono text-ellipsis whitespace-nowrap">
          {item.created_at
            ? `Created ${formatDateTime(item.created_at)}`
            : item.id}
        </small>
      </div>
      <div className="grid gap-1 max-[1150px]:col-start-2 max-[720px]:col-start-2 max-[480px]:col-start-1">
        <span
          className={`text-admin-meta ${item.mfa_enrolled ? 'text-admin-success' : 'text-admin-warning'}`}
        >
          {item.mfa_enrolled ? 'MFA enrolled' : 'MFA not enrolled'}
        </span>
        <small className="text-admin-muted-subtle text-admin-label">
          {item.roles?.map((role) => formatLabel(role.name)).join(', ') ||
            'No role assigned'}
        </small>
      </div>
      {canManage && !isCurrent ? (
        <div className="grid grid-cols-[minmax(120px,1fr)_auto] items-center gap-2 max-[1150px]:col-start-3 max-[1150px]:row-span-2 max-[720px]:col-start-2 max-[720px]:row-auto max-[480px]:col-start-1">
          <select
            aria-label={`Role for ${item.display_name}`}
            value={item.roles?.[0]?.id ?? ''}
            onChange={(event) => void onRoleChange(item, event.target.value)}
            className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input p-admin-7 focus:shadow-admin-focus text-admin-note focus:bg-admin-surface-raised w-full min-w-0 border transition-[border-color,box-shadow,background] duration-120 outline-none"
          >
            {roles.map((role) => (
              <option key={role.id} value={role.id}>
                {formatLabel(role.name)}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => void onToggleStatus(item)}
            className={`py-admin-7 px-admin-9 text-admin-filter cursor-pointer rounded-md border bg-transparent ${item.status === 'active' ? 'text-admin-danger border-admin-danger-border' : 'text-admin-success border-admin-success-border'}`}
          >
            {item.status === 'active' ? 'Disable' : 'Activate'}
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-[minmax(120px,1fr)_auto] items-center gap-2 max-[1150px]:col-start-3 max-[1150px]:row-span-2 max-[720px]:col-start-2 max-[720px]:row-auto max-[480px]:col-start-1">
          <span className="text-admin-label text-admin-muted-subtle col-span-full text-right font-mono uppercase">
            {isCurrent ? 'Current identity' : 'View only'}
          </span>
        </div>
      )}
    </article>
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
