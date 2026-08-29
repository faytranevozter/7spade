import { NavLink, Outlet } from 'react-router'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from '../components/AdminBrand'

export function AdminLayout() {
  const { admin, signOut } = useAuth()

  if (!admin) return null

  return (
    <div className="grid min-h-screen grid-cols-1 md:grid-cols-[260px_1fr]">
      <aside className="border-admin-control-border bg-admin-control-surface static top-0 flex w-full flex-col border-b p-6 md:sticky md:h-screen md:border-r md:border-b-0 md:p-4.5">
        <AdminBrand subtitle="Seven Spade operations" />
        <nav
          aria-label="Admin navigation"
          className="mt-6 grid grid-cols-2 gap-1.5 sm:grid-cols-3 md:mt-12 md:grid-cols-1"
        >
          {admin.permissions.includes('dashboard.read') ? (
            <NavLink
              to="/overview"
              className={({ isActive }) =>
                `focus-visible:outline-admin-control-accent border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 ${isActive ? 'border-admin-control-accent bg-admin-control-accent-soft text-admin-control-ink' : 'text-admin-control-muted-nav border-transparent hover:text-white'}`
              }
            >
              Overview
            </NavLink>
          ) : null}
          {admin.permissions.includes('admins.read') ? (
            <div className="border-l-admin-accent/22 my-admin-6 gap-admin-2 py-admin-6 pl-admin-10 grid border-l pr-0">
              <span className="mb-admin-2 px-admin-7 text-admin-tiny text-admin-control-muted-label font-mono tracking-[0.12em] uppercase">
                Access control
              </span>
              <NavLink
                to="/administrators"
                className={({ isActive }) =>
                  `rounded-admin-control p-admin-7 text-admin-action no-underline ${isActive ? 'bg-admin-accent/9 text-admin-accent-bright' : 'hover:text-admin-ink text-admin-control-muted-nav hover:bg-white/3'}`
                }
              >
                Administrators
              </NavLink>
              <NavLink
                to="/roles"
                className={({ isActive }) =>
                  `rounded-admin-control p-admin-7 text-admin-action no-underline ${isActive ? 'bg-admin-accent/9 text-admin-accent-bright' : 'hover:text-admin-ink text-admin-control-muted-nav hover:bg-white/3'}`
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
                `focus-visible:outline-admin-control-accent border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 ${isActive ? 'border-admin-control-accent bg-admin-control-accent-soft text-admin-control-ink' : 'text-admin-control-muted-nav border-transparent hover:text-white'}`
              }
            >
              Users
            </NavLink>
          ) : null}
          {admin.permissions.includes('rooms.read') ? (
            <NavLink
              to="/rooms"
              className={({ isActive }) =>
                `focus-visible:outline-admin-control-accent border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 ${isActive ? 'border-admin-control-accent bg-admin-control-accent-soft text-admin-control-ink' : 'text-admin-control-muted-nav border-transparent hover:text-white'}`
              }
            >
              Rooms
            </NavLink>
          ) : null}
          {admin.permissions.includes('games.read') ? (
            <NavLink
              to="/games"
              className={({ isActive }) =>
                `focus-visible:outline-admin-control-accent border-l-2 px-3 py-2.5 no-underline focus-visible:outline-2 ${isActive ? 'border-admin-control-accent bg-admin-control-accent-soft text-admin-control-ink' : 'text-admin-control-muted-nav border-transparent hover:text-white'}`
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
            <span className="text-admin-control-muted-nav border-l-2 border-transparent px-3 py-2.5">
              Content
            </span>
          ) : null}
          {admin.permissions.includes('skins.read') ? (
            <NavLink
              to="/skins"
              className={({ isActive }) =>
                `border-l-2 px-3 py-2.5 no-underline ${isActive ? 'border-admin-control-accent text-admin-control-ink' : 'text-admin-control-muted-nav border-transparent'}`
              }
            >
              Skins
            </NavLink>
          ) : null}
          {admin.permissions.includes('audit.read') ? (
            <span className="text-admin-control-muted-nav border-l-2 border-transparent px-3 py-2.5">
              Audit
            </span>
          ) : null}
        </nav>
        <div className="border-admin-control-border mt-6 grid gap-1.5 border-t pt-4.5 md:mt-auto">
          <small className="text-admin-control-muted-subtle">
            Signed in as
          </small>
          <strong className="font-bold text-white">{admin.display_name}</strong>
          <button
            onClick={signOut}
            className="text-admin-control-muted-strong focus-visible:outline-admin-control-accent mt-2 cursor-pointer border border-[#394552] bg-transparent p-2.5 transition-colors hover:bg-white/5 focus-visible:outline-2"
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
