import { type ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router'
import { useAuth } from '../hooks/useAuth'
import { AdminBrand } from '../components/AdminBrand'

export function AdminLayout() {
  const { admin, signOut } = useAuth()

  if (!admin) return null

  return (
    <div className="bg-admin-canvas min-h-screen md:grid md:grid-cols-[17rem_minmax(0,1fr)]">
      <aside className="border-admin-border-subtle bg-admin-surface-translucent flex flex-col border-b px-5 py-5 backdrop-blur-sm md:sticky md:top-0 md:h-screen md:border-r md:border-b-0 md:px-5 md:py-7">
        <AdminBrand subtitle="Seven Spade operations" />
        <nav
          aria-label="Admin navigation"
          className="mt-8 grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-1"
        >
          {admin.permissions.includes('dashboard.read') ? (
            <NavItem to="/overview">Overview</NavItem>
          ) : null}
          {admin.permissions.includes('admins.read') ? (
            <section
              className="border-admin-border-divider mt-3 grid gap-1 border-t pt-4 md:mt-4"
              aria-label="Access control"
            >
              <p className="text-admin-muted-subtle text-admin-note px-3 font-mono tracking-[0.13em] uppercase">
                Access control
              </p>
              <NavItem to="/administrators">Administrators</NavItem>
              <NavItem to="/roles">Roles &amp; permissions</NavItem>
            </section>
          ) : null}
          {admin.permissions.includes('users.read') ? (
            <NavItem to="/users">Users</NavItem>
          ) : null}
          {admin.permissions.includes('rooms.read') ? (
            <NavItem to="/rooms">Rooms</NavItem>
          ) : null}
          {admin.permissions.includes('games.read') ? (
            <NavItem to="/games">Games</NavItem>
          ) : null}
          {admin.permissions.includes('achievements.read') ? (
            <NavItem to="/achievements">Achievements</NavItem>
          ) : null}
          {admin.permissions.includes('skins.read') ? (
            <NavItem to="/skins">Skins</NavItem>
          ) : null}
        </nav>
        <div className="border-admin-border-divider mt-6 grid gap-1 border-t pt-5 md:mt-auto">
          <span className="text-admin-muted-subtle text-admin-note font-mono tracking-[0.13em] uppercase">
            Signed in
          </span>
          <strong className="text-admin-ink text-admin-field truncate">
            {admin.display_name}
          </strong>
          <button
            onClick={signOut}
            className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent-bright rounded-admin-input text-admin-field focus-visible:outline-admin-accent mt-3 cursor-pointer border bg-transparent px-3 py-2.5 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2"
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="p-5 sm:p-8 md:p-10 lg:p-14">
        <Outlet />
      </main>
    </div>
  )
}

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `rounded-admin-input text-admin-field focus-visible:outline-admin-accent px-3 py-2.5 font-medium no-underline transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 ${isActive ? 'bg-admin-accent-soft text-admin-accent-bright' : 'text-admin-muted hover:bg-admin-surface-raised hover:text-admin-ink'}`
      }
    >
      {children}
    </NavLink>
  )
}
