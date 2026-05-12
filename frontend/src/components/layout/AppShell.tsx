import { Link, NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../lib/auth";

const navLink = ({ isActive }: { isActive: boolean }) =>
  [
    "px-3 py-1.5 rounded-md text-sm transition-colors",
    isActive
      ? "bg-ink-100 text-ink-950"
      : "text-ink-600 hover:text-ink-950",
  ].join(" ");

export function AppShell() {
  const { user, logout } = useAuth();
  return (
    <div className="min-h-full">
      <header className="border-b border-ink-200 bg-white/70 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <Link to="/app" className="flex items-center gap-2">
            <span className="inline-block h-6 w-6 rounded bg-ink-950" />
            <span className="text-sm font-semibold tracking-tight">
              trippy<span className="text-ink-400">.ai</span>
            </span>
          </Link>
          <nav className="flex items-center gap-1">
            <NavLink to="/app" end className={navLink}>
              Dashboard
            </NavLink>
            <NavLink to="/app/trips" className={navLink}>
              Trips
            </NavLink>
            <NavLink to="/app/profile" className={navLink}>
              Profile
            </NavLink>
          </nav>
          <div className="flex items-center gap-3">
            <span className="text-sm text-ink-500">
              {user?.displayName || user?.username}
            </span>
            <button
              onClick={logout}
              className="text-sm text-ink-500 hover:text-ink-950"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-10">
        <Outlet />
      </main>
    </div>
  );
}
