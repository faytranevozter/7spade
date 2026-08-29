import { useEffect, useState } from 'react'
import {
  getAdmins,
  getRoles,
  inviteAdmin,
  setAdminRoles,
  setAdminStatus,
  type Admin,
  type Role,
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
    Promise.all([getAdmins(token), getRoles(token)])
      .then(([nextAdmins, nextRoles]) => {
        if (cancelled) return
        setAdmins(nextAdmins ?? [])
        setRoles(nextRoles ?? [])
        setInviteRoleID((current) => current || nextRoles[0]?.id || '')
        setMessage('')
      })
      .catch((error: unknown) => {
        if (!cancelled)
          setMessage(
            error instanceof Error
              ? error.message
              : 'Failed to load administrators',
          )
      })
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
      setMessage(`Invitation created for ${response.invitation.email}`)
      setInviteEmail('')
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : 'Failed to invite administrator',
      )
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
    } catch (error) {
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
    } catch (error) {
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
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Access control / Identities
          </p>
          <h1
            id="admins-heading"
            className="text-admin-ink-strong mt-[0.55rem] mr-0 mb-[0.65rem] ml-0 text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            Administrators
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Invite operators, review their access posture, and manage identity
            lifecycle independently from role policy.
          </p>
        </div>
        <div className="border-admin-ink/11 bg-admin-surface/80 grid min-w-87.5 grid-cols-3 overflow-hidden rounded-xl border max-[720px]:min-w-0 max-[480px]:grid-cols-1">
          <div className="border-admin-ink/9 border-r p-[0.85rem] last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright block font-mono text-[1.25rem] font-medium">
              {admins.length}
            </strong>
            <span className="text-admin-muted-subtle mt-[0.18rem] block text-[0.6rem]">
              Total identities
            </span>
          </div>
          <div className="border-admin-ink/9 border-r p-[0.85rem] last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright block font-mono text-[1.25rem] font-medium">
              {activeCount}
            </strong>
            <span className="text-admin-muted-subtle mt-[0.18rem] block text-[0.6rem]">
              Active
            </span>
          </div>
          <div className="border-admin-ink/9 border-r p-[0.85rem] last:border-r-0 max-[480px]:border-r-0 max-[480px]:border-b max-[480px]:last:border-b-0">
            <strong className="text-admin-accent-bright block font-mono text-[1.25rem] font-medium">
              {mfaCount}
            </strong>
            <span className="text-admin-muted-subtle mt-[0.18rem] block text-[0.6rem]">
              MFA enrolled
            </span>
          </div>
        </div>
      </header>

      {message ? <Notice variant="success">{message}</Notice> : null}
      {inviteToken ? (
        <CredentialNotice
          eyebrow="One-time invitation credential"
          credential={`Token: ${inviteToken}`}
          description="Store this securely. It is displayed only for this response."
          onDismiss={() => setInviteToken('')}
        />
      ) : null}

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,350px)] items-start gap-6 max-[900px]:grid-cols-1">
        <main>
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-[14px] border p-5">
            <SectionHeading
              eyebrow="Identity directory"
              title="Administrator list"
              id="administrator-list-heading"
              meta={`${visibleAdmins.length} shown`}
            />
            <div className="my-[1.1rem] grid grid-cols-[minmax(0,1fr)_180px] gap-[0.8rem] max-[720px]:grid-cols-1">
              <FilterField label="Search administrators">
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Name, email, or administrator ID"
                  className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                />
              </FilterField>
              <FilterField label="Account status">
                <select
                  value={status}
                  onChange={(event) => setStatus(event.target.value)}
                  className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                >
                  <option value="">Any status</option>
                  <option value="active">Active</option>
                  <option value="disabled">Disabled</option>
                  <option value="invited">Invited</option>
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
            <div className="grid gap-[0.55rem]">
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
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-[14px] border p-5">
            <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
              Provision access
            </p>
            <h2 className="text-admin-ink-strong my-[0.4rem] text-[1.2rem]">
              Invite administrator
            </h2>
            <p className="text-admin-muted m-0 text-[0.75rem] leading-[1.55]">
              New administrators receive one initial role. Permission policy is
              managed separately under Roles & permissions.
            </p>
            {canManage ? (
              <form
                onSubmit={handleInvite}
                className="border-admin-ink/9 mt-[1.2rem] grid gap-[0.9rem] border-t pt-[1.1rem]"
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
                    className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                  />
                </FilterField>
                <FilterField label="Initial role">
                  <select
                    id="invite-role"
                    aria-label="Role"
                    value={inviteRoleID}
                    onChange={(event) => setInviteRoleID(event.target.value)}
                    className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border px-3 py-[0.7rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
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
                  className="border-admin-accent bg-admin-accent cursor-pointer rounded-[7px] border p-[0.68rem] text-[0.72rem] font-semibold text-[#1a1204] disabled:cursor-not-allowed disabled:opacity-35"
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
    <article className="border-admin-ink/9 hover:border-admin-accent/30 grid grid-cols-[44px_minmax(200px,1fr)_minmax(130px,0.55fr)_minmax(190px,auto)] items-center gap-[0.85rem] rounded-[9px] border p-[0.8rem] transition-[border-color,background] duration-120 hover:bg-white/2 max-[1150px]:grid-cols-[44px_minmax(190px,1fr)_minmax(180px,auto)] max-[720px]:grid-cols-[40px_minmax(0,1fr)] max-[480px]:grid-cols-1">
      <span className="text-admin-ink grid size-10 place-items-center rounded-full bg-[#235c36] text-[0.72rem] font-semibold">
        {initials(item.display_name)}
      </span>
      <div>
        <div className="flex flex-wrap items-center gap-[0.4rem]">
          <strong className="text-admin-ink text-[0.82rem]">
            {item.display_name}
          </strong>
          {isCurrent ? (
            <span className="border-admin-accent/40 rounded-full border px-[0.42rem] py-[0.18rem] font-mono text-[0.5rem] text-[#e0b45e] uppercase">
              You
            </span>
          ) : null}
          <span
            className={`rounded-full border px-[0.42rem] py-[0.18rem] font-mono text-[0.5rem] uppercase ${item.status === 'active' ? 'text-admin-success border-[#2d7a46]/55' : item.status === 'disabled' ? 'text-admin-danger border-[#c0392b]/50' : 'border-admin-ink/15 text-[#e0b45e]'}`}
          >
            {formatLabel(item.status)}
          </span>
        </div>
        <p className="text-admin-muted my-[0.22rem] text-[0.68rem]">
          {item.email}
        </p>
        <small className="block max-w-75 overflow-hidden font-mono text-[0.54rem] text-ellipsis whitespace-nowrap text-[#60645e]">
          {item.created_at
            ? `Created ${formatDateTime(item.created_at)}`
            : item.id}
        </small>
      </div>
      <div className="grid gap-1 max-[1150px]:col-start-2 max-[720px]:col-start-2 max-[480px]:col-start-1">
        <span
          className={`text-[0.65rem] ${item.mfa_enrolled ? 'text-admin-success' : 'text-[#e0b45e]'}`}
        >
          {item.mfa_enrolled ? 'MFA enrolled' : 'MFA not enrolled'}
        </span>
        <small className="text-admin-muted-subtle text-[0.6rem]">
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
            className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border p-[0.55rem] text-[0.68rem] transition-[border-color,box-shadow,background] duration-120 outline-none focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
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
            className={`cursor-pointer rounded-md border bg-transparent px-[0.65rem] py-[0.55rem] text-[0.64rem] ${item.status === 'active' ? 'text-admin-danger border-[#c0392b]/55' : 'text-admin-success border-[#2d7a46]/60'}`}
          >
            {item.status === 'active' ? 'Disable' : 'Activate'}
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-[minmax(120px,1fr)_auto] items-center gap-2 max-[1150px]:col-start-3 max-[1150px]:row-span-2 max-[720px]:col-start-2 max-[720px]:row-auto max-[480px]:col-start-1">
          <span className="col-span-full text-right font-mono text-[0.58rem] text-[#666b64] uppercase">
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
