import { useEffect, useState } from 'react'
import {
  getPermissions,
  getRoles,
  updateRolePermissions,
  type Permission,
  type Role,
} from '../api/auth'
import { LoadingState, Notice, ReadOnlyNotice } from '../components/Feedback'
import { SectionHeading } from '../components/InvestigationUI'
import { formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function RolesPage() {
  const { admin, token, expireSession } = useAuth()
  const [roles, setRoles] = useState<Role[]>([])
  const [permissions, setPermissions] = useState<Permission[]>([])
  const [selectedRoleID, setSelectedRoleID] = useState('')
  const [query, setQuery] = useState('')
  const [message, setMessage] = useState('')
  const [messageTone, setMessageTone] = useState<'success' | 'error'>('error')
  const [saving, setSaving] = useState(false)
  const canManage = admin?.permissions.includes('admins.manage') ?? false

  useEffect(() => {
    if (!token || !admin?.permissions.includes('admins.read')) return
    let cancelled = false
    Promise.all([
      getRoles(token),
      canManage ? getPermissions(token) : Promise.resolve([]),
    ])
      .then(([nextRoles, nextPermissions]) => {
        if (cancelled) return
        setRoles(nextRoles ?? [])
        setPermissions(nextPermissions ?? [])
        setSelectedRoleID((current) => current || nextRoles[0]?.id || '')
      })
      .catch((error: unknown) => {
        if (!cancelled) setMessageTone('error')
        if (!cancelled)
          setMessage(
            error instanceof Error
              ? error.message
              : 'Failed to load roles and permissions',
          )
      })
    return () => {
      cancelled = true
    }
  }, [admin, canManage, token])

  function togglePermission(permission: string) {
    setRoles((current) =>
      current.map((role) =>
        role.id !== selectedRoleID
          ? role
          : {
              ...role,
              permissions: role.permissions.includes(permission)
                ? role.permissions.filter((item) => item !== permission)
                : [...role.permissions, permission],
            },
      ),
    )
  }

  async function saveRole() {
    const role = roles.find((item) => item.id === selectedRoleID)
    if (!token || !role) return
    setSaving(true)
    try {
      await updateRolePermissions(token, role.id, role.permissions)
      if (admin?.roles?.some((assignedRole) => assignedRole.id === role.id)) {
        expireSession('Your permissions changed. Sign in again to continue.')
        return
      }
      setMessage(`${formatLabel(role.name)} permissions updated`)
      setMessageTone('success')
    } catch (error) {
      setMessageTone('error')
      setMessage(
        error instanceof Error
          ? error.message
          : 'Failed to update role permissions',
      )
    } finally {
      setSaving(false)
    }
  }

  if (!admin) return null
  const selectedRole = roles.find((role) => role.id === selectedRoleID)
  const normalizedQuery = query.trim().toLowerCase()
  const visiblePermissions = permissions.filter(
    (permission) =>
      !normalizedQuery ||
      `${permission.name} ${permission.description}`
        .toLowerCase()
        .includes(normalizedQuery),
  )
  const permissionGroups = groupPermissions(visiblePermissions)

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="roles-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[720px]:flex-col max-[720px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-label m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Access control / Policy
          </p>
          <h1
            id="roles-heading"
            className="text-admin-ink-strong my-admin-7 mb-admin-9 text-admin-investigation-hero font-medium"
          >
            Roles & permissions
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Define reusable authorization policy separately from the
            administrators who receive it.
          </p>
        </div>
        <div className="border-admin-accent/32 bg-admin-accent/7 rounded-admin-preview py-admin-14 min-w-47.5 border px-4 max-[720px]:min-w-0">
          <strong className="text-admin-accent-bright text-admin-stat block font-mono">
            {roles.length}
          </strong>
          <span className="text-admin-ink-soft text-admin-control block">
            defined roles
          </span>
          <small className="text-admin-muted-subtle mt-admin-3 text-admin-session block">
            {permissions.length} available permissions
          </small>
        </div>
      </header>
      {message ? <Notice variant={messageTone}>{message}</Notice> : null}

      <div className="mt-6 grid grid-cols-[minmax(230px,290px)_minmax(0,1fr)] items-start gap-6 max-[900px]:grid-cols-1">
        <aside
          className="sticky top-6 max-[900px]:static"
          aria-labelledby="role-directory-heading"
        >
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel border p-5">
            <SectionHeading
              eyebrow="Policy directory"
              title="Roles"
              id="role-directory-heading"
              meta={roles.length}
            />
            <div className="gap-admin-6 mt-4 grid max-[900px]:grid-cols-2 max-[480px]:grid-cols-1">
              {roles.map((role) => (
                <button
                  key={role.id}
                  type="button"
                  onClick={() => setSelectedRoleID(role.id)}
                  className={
                    role.id === selectedRoleID
                      ? 'border-admin-accent/52 bg-admin-accent/7 hover:border-admin-accent/28 grid cursor-pointer rounded-lg border p-3 text-left text-inherit'
                      : 'border-admin-ink/9 hover:border-admin-accent/28 grid cursor-pointer rounded-lg border bg-transparent p-3 text-left text-inherit'
                  }
                >
                  <span className="text-admin-ink text-admin-action font-semibold">
                    {formatLabel(role.name)}
                  </span>
                  <small className="text-admin-accent mt-admin-2 text-admin-xs font-mono">
                    {role.permissions.length} permissions
                  </small>
                  <p className="text-admin-muted-subtle mt-admin-6 text-admin-help mb-0 leading-[1.4]">
                    {role.description || 'No role description provided.'}
                  </p>
                </button>
              ))}
            </div>
          </section>
        </aside>

        <main>
          <section className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel border p-5">
            {selectedRole ? (
              <>
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                      Permission mapping
                    </p>
                    <h2 className="text-admin-ink-strong my-admin-4 text-[1.35rem]">
                      {formatLabel(selectedRole.name)}
                    </h2>
                    <p className="text-admin-muted text-admin-field m-0">
                      {selectedRole.description ||
                        'Control the capabilities granted to this role.'}
                    </p>
                  </div>
                  <span className="text-admin-success px-admin-8 py-admin-3 border-admin-success-border text-admin-session rounded-full border font-mono">
                    {selectedRole.permissions.length} enabled
                  </span>
                </div>
                {canManage ? (
                  <>
                    <label className="mt-admin-16 block max-w-105">
                      <span className="sr-only">Search permissions</span>
                      <input
                        value={query}
                        onChange={(event) => setQuery(event.target.value)}
                        placeholder="Search permissions..."
                        className="border-admin-ink/15 bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input py-admin-10 focus:shadow-admin-focus placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised text-admin-field w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                      />
                    </label>
                    <div className="gap-admin-13 mt-4 grid grid-cols-2 max-[720px]:grid-cols-1">
                      {Object.entries(permissionGroups).map(
                        ([group, items]) => (
                          <PermissionGroup
                            key={group}
                            group={group}
                            permissions={items}
                            selected={selectedRole.permissions}
                            onToggle={togglePermission}
                          />
                        ),
                      )}
                    </div>
                    <div className="border-admin-ink/9 mt-admin-16 flex items-center justify-between gap-4 border-t pt-4 max-[720px]:flex-col max-[720px]:items-stretch">
                      <div>
                        <strong className="text-admin-ink-soft text-admin-field">
                          Review before saving
                        </strong>
                        <p className="text-admin-muted-subtle mt-admin-2 text-admin-caption mb-0 max-w-120">
                          Changes affect every administrator assigned to this
                          role and may revoke active authorization state.
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => void saveRole()}
                        disabled={saving}
                        className="border-admin-accent bg-admin-accent rounded-admin-input text-admin-button-ink px-admin-10 py-admin-10 text-admin-field min-w-47.5 cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-35"
                      >
                        {saving
                          ? 'Saving...'
                          : `Save ${selectedRole.name} permissions`}
                      </button>
                    </div>
                  </>
                ) : (
                  <div>
                    <ReadOnlyNotice title="Read-only role access">
                      You can inspect role assignments but cannot change
                      permission policy.
                    </ReadOnlyNotice>
                    <div className="mt-admin-13 gap-admin-5 flex flex-wrap">
                      {selectedRole.permissions.map((permission) => (
                        <span
                          key={permission}
                          className="border-admin-ink/10 text-admin-muted py-admin-3 text-admin-xs rounded-full border px-2 font-mono"
                        >
                          {permission}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </>
            ) : (
              <LoadingState>No roles are available.</LoadingState>
            )}
          </section>
        </main>
      </div>
    </section>
  )
}

function PermissionGroup({
  group,
  permissions,
  selected,
  onToggle,
}: {
  group: string
  permissions: Permission[]
  selected: string[]
  onToggle: (permission: string) => void
}) {
  return (
    <fieldset className="border-admin-ink/10 rounded-admin-rule min-w-0 border p-3">
      <legend className="px-admin-4 text-admin-caption text-admin-warning font-mono uppercase">
        {formatLabel(group)}
      </legend>
      <div className="gap-admin-4 grid">
        {permissions.map((permission) => (
          <label
            key={permission.name}
            className="gap-admin-8 p-admin-7 flex cursor-pointer items-start rounded-md hover:bg-white/3"
          >
            <input
              type="checkbox"
              aria-label={`${permission.name} for ${group}`}
              checked={selected.includes(permission.name)}
              onChange={() => onToggle(permission.name)}
              className="accent-admin-accent mt-admin-1 size-4.5 shrink-0 cursor-pointer disabled:cursor-not-allowed"
            />
            <span className="block min-w-0">
              <strong className="text-admin-ink-soft text-admin-meta-small block min-w-0 font-mono font-medium">
                {permission.name}
              </strong>
              <small className="text-admin-muted-subtle text-admin-label mt-[0.24rem] block min-w-0 leading-[1.35]">
                {permission.description}
              </small>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}

function groupPermissions(permissions: Permission[]) {
  return permissions.reduce<Record<string, Permission[]>>(
    (groups, permission) => {
      const group = permission.name.split('.')[0] || 'other'
      groups[group] = [...(groups[group] ?? []), permission]
      return groups
    },
    {},
  )
}
