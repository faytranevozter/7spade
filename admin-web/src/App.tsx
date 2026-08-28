import { HashRouter, Navigate, Route, Routes } from 'react-router'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { LoginPage } from './pages/LoginPage'
import { MFAChallengePage } from './pages/MFAChallengePage'
import { AdminLayout } from './pages/AdminLayout'
import { OverviewPage } from './pages/OverviewPage'
import { AdministratorsPage } from './pages/AdministratorsPage'
import { UsersPage } from './pages/UsersPage'
import { UserDetailPage } from './pages/UserDetailPage'
import { AuditEventPage } from './pages/AuditEventPage'

function RequirePermission({ permission, children }: { permission: string; children: React.ReactNode }) {
  const { admin } = useAuth()
  if (!admin?.permissions.includes(permission)) return <Navigate to="/overview" replace />
  return <>{children}</>
}

function AppRoutes() {
  const { admin, challengeToken, isLoading } = useAuth()

  if (isLoading) return <main className="min-h-screen grid place-items-center text-[#91a0b2]">Checking administrator session...</main>
  if (!admin && challengeToken) return <MFAChallengePage />
  if (!admin) return <LoginPage />

  return (
    <Routes>
      <Route element={<AdminLayout />}>
        <Route path="/" element={<Navigate to="/overview" replace />} />
        <Route path="/overview" element={<OverviewPage />} />
        <Route path="/administrators" element={
          <RequirePermission permission="admins.read"><AdministratorsPage /></RequirePermission>
        } />
        <Route path="/users" element={
          <RequirePermission permission="users.read"><UsersPage /></RequirePermission>
        } />
        <Route path="/users/:id" element={
          <RequirePermission permission="users.read"><UserDetailPage /></RequirePermission>
        } />
        <Route path="/audit-events/:id" element={
          <RequirePermission permission="audit.read"><AuditEventPage /></RequirePermission>
        } />
        <Route path="*" element={<Navigate to="/overview" replace />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <HashRouter>
        <AppRoutes />
      </HashRouter>
    </AuthProvider>
  )
}
