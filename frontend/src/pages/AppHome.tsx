import { Link } from "react-router-dom";
import { Card } from "../components/ui/Card";
import { useAuth } from "../lib/auth";

export function AppHome() {
  const { user } = useAuth();
  return (
    <div>
      <h1 className="title-gradient text-3xl font-semibold tracking-tight">
        Hello, {user?.displayName || user?.username}.
      </h1>
      <p className="mt-2 text-ink-500">
        Pick a trip, or start a new one.
      </p>
      <div className="mt-10 grid gap-4 md:grid-cols-3">
        <Link to="/app/trips/new">
          <Card className="h-full p-6 transition-colors hover:border-ink-400">
            <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
              New
            </p>
            <h2 className="mt-2 text-lg font-medium">Create a trip</h2>
            <p className="mt-2 text-sm text-ink-500">
              Title, dates, visibility. Invite friends after.
            </p>
          </Card>
        </Link>
        <Link to="/app/trips">
          <Card className="h-full p-6 transition-colors hover:border-ink-400">
            <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
              Browse
            </p>
            <h2 className="mt-2 text-lg font-medium">Your trips</h2>
            <p className="mt-2 text-sm text-ink-500">
              Past and upcoming, all in one place.
            </p>
          </Card>
        </Link>
        <Card className="h-full p-6 opacity-60">
          <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
            Soon
          </p>
          <h2 className="mt-2 text-lg font-medium">Friends</h2>
          <p className="mt-2 text-sm text-ink-500">
            Lands in milestone&nbsp;1.
          </p>
        </Card>
      </div>
    </div>
  );
}
