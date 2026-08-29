import { NavLink, Outlet } from 'react-router'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from '../components/AdminBrand'

export function AdminLayout() {
  const { admin, signOut } = useAuth()

  if (!admin) return null

  return (
    <div className="grid min-h-screen grid-cols-1 md:grid-cols-[260px_1fr]">
      <aside className="static top-0 flex w-full flex-col border-b border-[#28323d] bg-[#0c1117] p-6 md:sticky md:h-screen md:border-r md:border-b-0 md:p-4.5">
        <AdminBrand subtitle="Seven Spade operations" />
        <nav
          aria-label="Admin navigation"
          className="mt-6 grid grid-cols-2 gap-1.5 sm:grid-cols-3 md:mt-12 md:grid-cols-1"
        >
          {admin.permissions.includes('dashboard.read') ? (
            <NavLink
              to="/overview"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'border-[#4dd0b5] bg-[#4dd0b5]/5 text-[#eafbf7]' : 'border-transparent text-[#7f8c9b] hover:text-white'}`
              }
            >
              Overview
            </NavLink>
          ) : null}
          {admin.permissions.includes('admins.read') ? (
            <div className="border-l-admin-accent/22 my-[0.45rem] grid gap-[0.2rem] border-l py-[0.45rem] pr-0 pl-[0.7rem]">
              <span className="mb-[0.2rem] px-[0.55rem] font-mono text-[0.56rem] tracking-[0.12em] text-[#6f736c] uppercase">
                Access control
              </span>
              <NavLink
                to="/administrators"
                className={({ isActive }) =>
                  `rounded-[5px] p-[0.55rem] text-[0.78rem] no-underline ${isActive ? 'bg-admin-accent/9 text-admin-accent-bright' : 'hover:text-admin-ink text-[#7f8c9b] hover:bg-white/3'}`
                }
              >
                Administrators
              </NavLink>
              <NavLink
                to="/roles"
                className={({ isActive }) =>
                  `rounded-[5px] p-[0.55rem] text-[0.78rem] no-underline ${isActive ? 'bg-admin-accent/9 text-admin-accent-bright' : 'hover:text-admin-ink text-[#7f8c9b] hover:bg-white/3'}`
                }
              >
                Roles & permissions
              </NavLink>
            </div>
          ) : null}
          {admin.permissions.includes('users.read') ? (
            <NavLink
              to="/users"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'border-[#4dd0b5] bg-[#4dd0b5]/5 text-[#eafbf7]' : 'border-transparent text-[#7f8c9b] hover:text-white'}`
              }
            >
              Users
            </NavLink>
          ) : null}
          {admin.permissions.includes('rooms.read') ? (
            <NavLink
              to="/rooms"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'border-[#4dd0b5] bg-[#4dd0b5]/5 text-[#eafbf7]' : 'border-transparent text-[#7f8c9b] hover:text-white'}`
              }
            >
              Rooms
            </NavLink>
          ) : null}
          {admin.permissions.includes('games.read') ? (
            <NavLink
              to="/games"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 focus-visible:outline-[#4dd0b5] ${isActive ? 'border-[#4dd0b5] bg-[#4dd0b5]/5 text-[#eafbf7]' : 'border-transparent text-[#7f8c9b] hover:text-white'}`
              }
            >
              Games
            </NavLink>
          ) : null}
          {admin.permissions.some((permission) =>
            [
              'seasons.read',
              'events.read',
              'achievements.read',
              'skins.read',
            ].includes(permission),
          ) ? (
            <span className="border-l-2 border-transparent px-3 py-2.5 text-[#7f8c9b]">
              Content
            </span>
          ) : null}
          {admin.permissions.includes('skins.read') ? (
            <NavLink
              to="/skins"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline ${isActive ? 'border-[#4dd0b5] text-[#eafbf7]' : 'border-transparent text-[#7f8c9b]'}`
              }
            >
              Skins
            </NavLink>
          ) : null}
          {admin.permissions.includes('audit.read') ? (
            <span className="border-l-2 border-transparent px-3 py-2.5 text-[#7f8c9b]">
              Audit
            </span>
          ) : null}
        </nav>
        <div className="mt-6 grid gap-1.5 border-t border-[#28323d] pt-4.5 md:mt-auto">
          <small className="text-[#8493a5]">Signed in as</small>
          <strong className="font-bold text-white">{admin.display_name}</strong>
          <button
            onClick={signOut}
            className="mt-2 cursor-pointer border border-[#394552] bg-transparent p-2.5 text-[#aeb8c4] transition-colors hover:bg-white/5 focus-visible:outline-2 focus-visible:outline-[#4dd0b5]"
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
