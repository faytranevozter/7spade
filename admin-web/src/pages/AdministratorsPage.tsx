import { useEffect, useState } from 'react'
import {
  getAdmins,
  getPermissions,
  getRoles,
  inviteAdmin,
  setAdminRoles,
  setAdminStatus,
  updateRolePermissions,
  type Admin,
  type Permission,
  type Role,
} from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function AdministratorsPage() {
  const { admin, token, expireSession } = useAuth()
  const [adminsList, setAdminsList] = useState<Admin[]>([])
  const [rolesList, setRolesList] = useState<Role[]>([])
  const [permissionsList, setPermissionsList] = useState<Permission[]>([])
  const [adminMessage, setAdminMessage] = useState('')
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRoleId, setInviteRoleId] = useState('')
  const [generatedInviteLink, setGeneratedInviteLink] = useState('')

  useEffect(() => {
    if (!token || !admin?.permissions.includes('admins.read')) return
    getAdmins(token)
      .then(setAdminsList)
      .catch((requestError: unknown) => setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to load administrators'))
    getRoles(token)
      .then((roles) => {
        setRolesList(roles)
        if (roles.length > 0) setInviteRoleId(roles[0].id)
      })
      .catch(() => {})
    if (admin.permissions.includes('admins.manage')) {
      getPermissions(token).then(setPermissionsList).catch(() => {})
    }
  }, [admin, token])

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault()
    if (!token || !inviteEmail || !inviteRoleId) return
    setAdminMessage('')
    try {
      const res = await inviteAdmin(token, inviteEmail, inviteRoleId)
      setGeneratedInviteLink(`Token: ${res.token}`)
      setAdminMessage(`Invitation created for ${res.invitation.email}`)
      setInviteEmail('')
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to invite administrator')
    }
  }

  async function handleToggleStatus(targetAdmin: Admin) {
    if (!token) return
    const nextStatus = targetAdmin.status === 'active' ? 'disabled' : 'active'
    if (!window.confirm(`Are you sure you want to ${nextStatus === 'disabled' ? 'disable' : 'activate'} ${targetAdmin.email}?`)) return
    try {
      await setAdminStatus(token, targetAdmin.id, nextStatus)
      setAdminsList((list) => list.map((a) => a.id === targetAdmin.id ? { ...a, status: nextStatus } : a))
      setAdminMessage(`Administrator ${targetAdmin.email} is now ${nextStatus}`)
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update administrator status')
    }
  }

  async function handleRoleChange(targetAdmin: Admin, roleId: string) {
    if (!token) return
    const roleIds = [roleId, ...(targetAdmin.roles?.slice(1).map((role) => role.id) ?? [])]
    try {
      await setAdminRoles(token, targetAdmin.id, roleIds)
      const targetRole = rolesList.find((r) => r.id === roleId)
      setAdminsList((list) => list.map((a) => a.id === targetAdmin.id ? { ...a, roles: targetRole ? [targetRole, ...(a.roles?.slice(1) ?? [])] : a.roles } : a))
      setAdminMessage('Administrator role updated')
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update administrator role')
    }
  }

  function toggleRolePermission(roleId: string, permission: string) {
    setRolesList((roles) => roles.map((role) => role.id !== roleId ? role : {
      ...role,
      permissions: role.permissions.includes(permission)
        ? role.permissions.filter((value) => value !== permission)
        : [...role.permissions, permission],
    }))
  }

  async function saveRolePermissions(role: Role) {
    if (!token) return
    try {
      await updateRolePermissions(token, role.id, role.permissions)
      if (admin?.roles?.some((assignedRole) => assignedRole.id === role.id)) {
        expireSession('Your permissions changed. Sign in again to continue.')
        return
      }
      setAdminMessage(`${role.name[0].toUpperCase()}${role.name.slice(1)} permissions updated`)
    } catch (requestError) {
      setAdminMessage(requestError instanceof Error ? requestError.message : 'Failed to update role permissions')
    }
  }

  if (!admin) return null

  return (
    <section id="administrators" className="mt-8 border border-[#28323d] bg-[#10161d] p-6" aria-labelledby="admins-heading">
      <div className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center">
        <div>
          <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">ACCESS CONTROL</p>
          <h2 id="admins-heading" className="mt-2 text-xl font-bold text-white">Administrators</h2>
        </div>
      </div>

      {admin.permissions.includes('admins.manage') ? (
        <form onSubmit={handleInvite} className="mt-5 p-4 border border-[#28323d] bg-[#0c1117] flex flex-col sm:flex-row gap-3 items-end">
          <div className="flex-1 w-full">
            <label htmlFor="invite-email" className="block text-xs font-mono text-[#8493a5] mb-1">Invite Email</label>
            <input
              id="invite-email"
              type="email"
              required
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              placeholder="admin@example.com"
              className="w-full bg-[#10161d] border border-[#28323d] text-white px-3 py-2 text-sm focus:outline-none focus:border-[#4dd0b5]"
            />
          </div>
          <div className="w-full sm:w-48">
            <label htmlFor="invite-role" className="block text-xs font-mono text-[#8493a5] mb-1">Role</label>
            <select
              id="invite-role"
              value={inviteRoleId}
              onChange={(e) => setInviteRoleId(e.target.value)}
              className="w-full bg-[#10161d] border border-[#28323d] text-white px-3 py-2 text-sm focus:outline-none focus:border-[#4dd0b5]"
            >
              {rolesList.map((r) => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
          </div>
          <button
            type="submit"
            className="bg-[#4dd0b5] text-[#0c1117] font-bold px-4 py-2 text-sm hover:bg-[#3dbca2] cursor-pointer"
          >
            Send Invite
          </button>
        </form>
      ) : null}

      {generatedInviteLink ? (
        <div className="mt-3 p-3 bg-[#4dd0b5]/10 border border-[#4dd0b5] text-[#4dd0b5] text-xs font-mono">
          {generatedInviteLink}
        </div>
      ) : null}

      {adminMessage ? <p role="status" className="text-[#4dd0b5] mt-4">{adminMessage}</p> : null}

      {admin.permissions.includes('admins.manage') ? (
        <div className="grid gap-3 mt-5" aria-label="Role permissions">
          {rolesList.map((role) => (
            <fieldset key={role.id} className="border border-[#28323d] p-4">
              <legend className="px-2 font-bold text-white">{role.name}</legend>
              <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-2">
                {permissionsList.map((permission) => (
                  <label key={permission.name} className="flex items-start gap-2 text-sm text-[#aeb8c4]">
                    <input
                      type="checkbox"
                      aria-label={`${permission.name} for ${role.name}`}
                      checked={role.permissions.includes(permission.name)}
                      onChange={() => toggleRolePermission(role.id, permission.name)}
                    />
                    <span><strong className="text-white">{permission.name}</strong><br />{permission.description}</span>
                  </label>
                ))}
              </div>
              <button
                type="button"
                onClick={() => saveRolePermissions(role)}
                className="mt-3 border border-[#4dd0b5] text-[#4dd0b5] bg-transparent px-3 py-1.5 text-xs cursor-pointer"
              >
                Save {role.name} permissions
              </button>
            </fieldset>
          ))}
        </div>
      ) : null}

      <div className="grid gap-3 mt-5">
        {adminsList.map((adm) => (
          <article key={adm.id} className="flex flex-col sm:flex-row gap-4 justify-between sm:items-center border border-[#28323d] p-4">
            <div>
              <div className="flex items-center gap-2">
                <strong className="text-white">{adm.display_name}</strong>
                <span className={`text-[10px] font-mono px-2 py-0.5 border ${adm.status === 'active' ? 'border-[#4dd0b5] text-[#4dd0b5]' : 'border-[#ff786f] text-[#ffaaa4]'}`}>
                  {adm.status.toUpperCase()}
                </span>
              </div>
              <p className="text-[#8493a5] text-sm mt-1">{adm.email}</p>
              <div className="flex flex-wrap gap-1 mt-2">
                {adm.roles?.map((r) => (
                  <span key={r.id} className="text-xs bg-[#28323d] text-[#eafbf7] px-2 py-0.5">
                    {r.name}
                  </span>
                ))}
              </div>
            </div>

            {admin.permissions.includes('admins.manage') && adm.id !== admin.id ? (
              <div className="flex items-center gap-3">
                <select
                  aria-label={`Role for ${adm.display_name}`}
                  value={adm.roles?.[0]?.id ?? ''}
                  onChange={(e) => handleRoleChange(adm, e.target.value)}
                  className="bg-[#0c1117] border border-[#28323d] text-white px-2 py-1 text-xs focus:outline-none focus:border-[#4dd0b5]"
                >
                  {rolesList.map((r) => (
                    <option key={r.id} value={r.id}>{r.name}</option>
                  ))}
                </select>
                <button
                  onClick={() => handleToggleStatus(adm)}
                  className={`border px-3 py-1 text-xs cursor-pointer ${adm.status === 'active' ? 'border-[#ff786f] text-[#ffaaa4]' : 'border-[#4dd0b5] text-[#4dd0b5]'}`}
                >
                  {adm.status === 'active' ? 'Disable' : 'Activate'}
                </button>
              </div>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  )
}
