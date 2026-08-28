import { RoomInvestigation } from '../components/RoomInvestigation'
import { useAuth } from '../hooks/useAuth'

export function RoomsPage() {
  const { token } = useAuth()
  if (!token) return null
  return <RoomInvestigation token={token} />
}
