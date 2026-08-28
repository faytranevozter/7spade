import { GameInvestigation } from '../components/GameInvestigation'
import { useAuth } from '../hooks/useAuth'
export function GamesPage() { const { token } = useAuth(); return token ? <GameInvestigation token={token} /> : null }
