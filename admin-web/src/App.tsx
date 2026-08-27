import { AdminShell } from './components/AdminShell'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { LoginPage } from './pages/LoginPage'
import { MFAChallengePage } from './pages/MFAChallengePage'

function AppShell() {
  const { admin, challengeToken, isLoading } = useAuth()

  if (isLoading) return <main className="min-h-screen grid place-items-center text-[#91a0b2]">Checking administrator session...</main>
  if (!admin && challengeToken) return <MFAChallengePage />
  if (!admin) return <LoginPage />
  return <AdminShell />
}

export default function App() {
  return <AuthProvider><AppShell /></AuthProvider>
}
