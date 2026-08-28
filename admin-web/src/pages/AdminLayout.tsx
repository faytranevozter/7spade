import { NavLink, Outlet } from 'react-router'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from '../components/AdminBrand'

export function AdminLayout() {
  const { admin, signOut } = useAuth()

  if (!admin) return null

  return (
    <div className="min-h-screen grid grid-cols-1 md:grid-cols-[260px_1fr]">
      <aside className="static md:sticky top-0 w-full md:h-screen flex flex-col border-b md:border-b-0 md:border-r border-[#28323d] bg-[#0c1117] p-6 md:p-4.5">
        <AdminBrand subtitle="Seven Spade operations" />
        <nav aria-label="Admin navigation" className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-1 gap-1.5 mt-6 md:mt-12">
          {admin.permissions.includes('dashboard.read') ? (
            <NavLink
              to="/overview"
              className={({ isActive }) => `py-2.5 px-3 no-underline border-l-2 focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'text-[#eafbf7] border-[#4dd0b5] bg-[#4dd0b5]/5' : 'text-[#7f8c9b] hover:text-white border-transparent'}`}
            >
              Overview
            </NavLink>
          ) : null}
          {admin.permissions.includes('admins.read') ? (
            <NavLink
              to="/administrators"
              className={({ isActive }) => `py-2.5 px-3 no-underline border-l-2 focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'text-[#eafbf7] border-[#4dd0b5] bg-[#4dd0b5]/5' : 'text-[#7f8c9b] hover:text-white border-transparent'}`}
            >
              Administrators
            </NavLink>
          ) : null}
          {admin.permissions.includes('users.read') ? (
            <NavLink
              to="/users"
              className={({ isActive }) => `py-2.5 px-3 no-underline border-l-2 focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'text-[#eafbf7] border-[#4dd0b5] bg-[#4dd0b5]/5' : 'text-[#7f8c9b] hover:text-white border-transparent'}`}
            >
              Users
            </NavLink>
          ) : null}
          {admin.permissions.includes('rooms.read') ? (
            <NavLink
              to="/rooms"
              className={({ isActive }) => `py-2.5 px-3 no-underline border-l-2 focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'text-[#eafbf7] border-[#4dd0b5] bg-[#4dd0b5]/5' : 'text-[#7f8c9b] hover:text-white border-transparent'}`}
            >
              Rooms
            </NavLink>
          ) : null}
          {admin.permissions.includes('games.read') ? (
            <NavLink
              to="/games"
              className={({ isActive }) => `py-2.5 px-3 no-underline border-l-2 focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'text-[#eafbf7] border-[#4dd0b5] bg-[#4dd0b5]/5' : 'text-[#7f8c9b] hover:text-white border-transparent'}`}
            >
              Games
            </NavLink>
          ) : null}
          {admin.permissions.some((permission) =>
            ['seasons.read', 'events.read', 'achievements.read', 'skins.read'].includes(permission),
          ) ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Content</span>
          ) : null}
          {admin.permissions.includes('audit.read') ? (
            <span className="text-[#7f8c9b] border-l-2 border-transparent py-2.5 px-3">Audit</span>
          ) : null}
        </nav>
        <div className="mt-6 md:mt-auto grid gap-1.5 border-t border-[#28323d] pt-4.5">
          <small className="text-[#8493a5]">Signed in as</small>
          <strong className="text-white font-bold">{admin.display_name}</strong>
          <button
            onClick={signOut}
            className="mt-2 border border-[#394552] bg-transparent text-[#aeb8c4] p-2.5 cursor-pointer hover:bg-white/5 transition-colors focus-visible:outline-2 focus-visible:outline-[#4dd0b5]"
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="p-6 md:p-12 lg:p-18">
        <Outlet />
      </main>
    </div>
  )
}
