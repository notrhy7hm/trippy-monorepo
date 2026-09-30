import { Link, NavLink, Outlet } from "react-router-dom";
import { House, Map, Users, UserRound, LogOut } from "lucide-react";
import { useAuth } from "../../lib/auth";

const navLink = ({ isActive }: { isActive: boolean }) =>
  [
    "flex h-9 w-8 shrink-0 items-center justify-center gap-2 rounded-md text-sm transition-colors sm:w-9 lg:w-auto lg:px-3",
    isActive
      ? "bg-ink-100 text-ink-950"
      : "text-ink-600 hover:text-ink-950",
  ].join(" ");

export function AppShell() {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-full">
      <header className="border-b border-ink-200 bg-white/70 backdrop-blur">
        <div className="mx-auto grid h-16 max-w-6xl grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 px-3 sm:gap-4 sm:px-6">
          <Link to="/app" aria-label="Trippy home" className="flex shrink-0 items-center gap-2">
            <span className="inline-block h-6 w-6 rounded bg-ink-950" aria-hidden="true" />
            <span className="hidden text-sm font-semibold sm:inline">
              trippy<span className="text-ink-400">.ai</span>
            </span>
          </Link>
          <nav aria-label="Main navigation" className="flex min-w-0 items-center justify-center gap-0 sm:gap-1">
            <NavLink to="/app" end className={navLink} aria-label="Dashboard" title="Dashboard">
              <House size={18} aria-hidden="true" />
              <span className="hidden lg:inline">Dashboard</span>
            </NavLink>
            <NavLink to="/app/trips" className={navLink} aria-label="Trips" title="Trips">
              <Map size={18} aria-hidden="true" />
              <span className="hidden lg:inline">Trips</span>
            </NavLink>
            <NavLink to="/app/friends" className={navLink} aria-label="Friends" title="Friends">
              <Users size={18} aria-hidden="true" />
              <span className="hidden lg:inline">Friends</span>
            </NavLink>
            <NavLink to="/app/profile" className={navLink} aria-label="Profile" title="Profile">
              <UserRound size={18} aria-hidden="true" />
              <span className="hidden lg:inline">Profile</span>
            </NavLink>
          </nav>
          <div className="flex min-w-0 items-center gap-1 sm:gap-2">
            <span className="max-w-[72px] truncate text-xs text-ink-600 sm:max-w-32 sm:text-sm lg:max-w-44" title={user?.username}>
              @{user?.username}
            </span>
            <button
              onClick={logout}
              aria-label="Sign out"
              title="Sign out"
              className="flex h-9 w-9 shrink-0 items-center justify-center gap-2 rounded-md text-sm text-ink-500 hover:bg-ink-100 hover:text-ink-950 xl:w-auto xl:px-2"
            >
              <LogOut size={18} aria-hidden="true" />
              <span className="hidden xl:inline">Sign out</span>
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
