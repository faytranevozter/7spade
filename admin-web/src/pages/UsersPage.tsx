import { UserInvestigation } from '../components/UserInvestigation'
import { useAuth } from '../hooks/useAuth'

export function UsersPage() {
  const { admin, token } = useAuth()
  if (!admin || !token) return null
  return <UserInvestigation token={token} canReadSensitive={admin.permissions.includes('users.sensitive.read')} />
}
