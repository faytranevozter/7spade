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
    } catch (error) {
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
    <section className="access-page roles-page" aria-labelledby="roles-heading">
      <header className="access-header">
        <div>
          <p className="eyebrow">Access control / Policy</p>
          <h1 id="roles-heading">Roles & permissions</h1>
          <p>
            Define reusable authorization policy separately from the
            administrators who receive it.
          </p>
        </div>
        <div className="role-policy-stat">
          <strong>{roles.length}</strong>
          <span>defined roles</span>
          <small>{permissions.length} available permissions</small>
        </div>
      </header>
      {message ? <Notice variant="success">{message}</Notice> : null}

      <div className="role-policy-layout">
        <aside
          className="role-directory"
          aria-labelledby="role-directory-heading"
        >
          <section className="access-panel">
            <SectionHeading
              eyebrow="Policy directory"
              title="Roles"
              id="role-directory-heading"
              meta={roles.length}
            />
            <div className="role-list">
              {roles.map((role) => (
                <button
                  key={role.id}
                  type="button"
                  onClick={() => setSelectedRoleID(role.id)}
                  className={
                    role.id === selectedRoleID
                      ? 'role-list-item selected'
                      : 'role-list-item'
                  }
                >
                  <span>{formatLabel(role.name)}</span>
                  <small>{role.permissions.length} permissions</small>
                  <p>{role.description || 'No role description provided.'}</p>
                </button>
              ))}
            </div>
          </section>
        </aside>

        <main className="permission-workspace">
          <section className="access-panel">
            {selectedRole ? (
              <>
                <div className="permission-heading">
                  <div>
                    <p className="eyebrow">Permission mapping</p>
                    <h2>{formatLabel(selectedRole.name)}</h2>
                    <p>
                      {selectedRole.description ||
                        'Control the capabilities granted to this role.'}
                    </p>
                  </div>
                  <span>{selectedRole.permissions.length} enabled</span>
                </div>
                {canManage ? (
                  <>
                    <label className="permission-search">
                      <span className="sr-only">Search permissions</span>
                      <input
                        value={query}
                        onChange={(event) => setQuery(event.target.value)}
                        placeholder="Search permissions..."
                        className="game-input"
                      />
                    </label>
                    <div className="permission-groups">
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
                    <div className="permission-save-bar">
                      <div>
                        <strong>Review before saving</strong>
                        <p>
                          Changes affect every administrator assigned to this
                          role and may revoke active authorization state.
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => void saveRole()}
                        disabled={saving}
                        className="primary-action"
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
                    <div className="readonly-permissions">
                      {selectedRole.permissions.map((permission) => (
                        <span key={permission}>{permission}</span>
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
    <fieldset className="permission-group">
      <legend>{formatLabel(group)}</legend>
      <div>
        {permissions.map((permission) => (
          <label key={permission.name} className="permission-option">
            <input
              type="checkbox"
              aria-label={`${permission.name} for ${group}`}
              checked={selected.includes(permission.name)}
              onChange={() => onToggle(permission.name)}
            />
            <span>
              <strong>{permission.name}</strong>
              <small>{permission.description}</small>
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
