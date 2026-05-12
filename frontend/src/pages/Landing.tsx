import { Link } from "react-router-dom";
import { Button } from "../components/ui/Button";

export function Landing() {
  return (
    <div className="mx-auto flex min-h-screen max-w-4xl flex-col items-center justify-center px-6 text-center">
      <p className="mb-6 text-xs uppercase tracking-[0.2em] text-ink-500">
        Trippy.ai · Collaborative trip planning
      </p>
      <h1 className="title-gradient text-5xl font-semibold tracking-tight md:text-7xl">
        Plan trips that actually happen.
      </h1>
      <p className="mt-6 max-w-xl text-base text-ink-600">
        A quiet, premium workspace for groups of friends — tasks, timeline,
        budget, and a trip-scoped AI assistant that proposes, never decides.
      </p>
      <div className="mt-10 flex items-center gap-3">
        <Link to="/register">
          <Button>Create an account</Button>
        </Link>
        <Link to="/login">
          <Button variant="secondary">Sign in</Button>
        </Link>
      </div>
    </div>
  );
}
