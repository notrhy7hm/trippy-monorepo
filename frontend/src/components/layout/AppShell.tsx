import { Link, NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../lib/auth";

const navLink = ({ isActive }: { isActive: boolean }) =>
  [
    "shrink-0 px-3 py-1.5 rounded-md text-sm transition-colors",
    isActive
      ? "bg-ink-100 text-ink-950"
      : "text-ink-600 hover:text-ink-950",
  ].join(" ");

export function AppShell() {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-full">
      <header className="border-b border-ink-200 bg-white/70 backdrop-blur">
        <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
          <Link to="/app" className="flex items-center gap-2">
            <span className="inline-block h-6 w-6 rounded bg-ink-950" />
            <span className="text-sm font-semibold tracking-tight">
              trippy<span className="text-ink-400">.ai</span>
            </span>
          </Link>
          <nav className="-mx-4 flex max-w-[calc(100vw-2rem)] items-center gap-1 overflow-x-auto px-4 pb-1 sm:mx-0 sm:max-w-none sm:px-0 sm:pb-0">
            <NavLink to="/app" end className={navLink}>
              Dashboard
            </NavLink>
            <NavLink to="/app/trips" className={navLink}>
              Trips
            </NavLink>
            <NavLink to="/app/friends" className={navLink}>
              Friends
            </NavLink>
            <NavLink to="/app/profile" className={navLink}>
              Profile
            </NavLink>
          </nav>
          <div className="flex min-w-0 items-center justify-between gap-3 lg:justify-end">
            <span className="min-w-0 truncate text-sm text-ink-500">
              {user?.displayName || user?.username}
            </span>
            <button
              onClick={logout}
              className="shrink-0 text-sm text-ink-500 hover:text-ink-950"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6 sm:px-6 sm:py-10">
        <Outlet />
      </main>
    </div>
  );
}
