import { Link } from "react-router-dom";
import { Button } from "../components/ui/Button";

export function Landing() {
  return (
    <div className="mx-auto flex min-h-screen max-w-4xl flex-col items-center justify-center px-4 py-10 text-center sm:px-6">
      <p className="mb-6 text-xs uppercase tracking-[0.2em] text-ink-500">
        Trippy.ai · Collaborative trip planning
      </p>
      <h1 className="title-gradient text-4xl font-semibold tracking-tight sm:text-5xl md:text-7xl">
        Plan trips that actually happen.
      </h1>
      <p className="mt-6 max-w-xl text-base text-ink-600">
        A quiet, premium workspace for groups of friends — tasks, timeline,
        budget, and a trip-scoped AI assistant that proposes, never decides.
      </p>
      <div className="mt-10 flex w-full flex-col items-stretch gap-3 sm:w-auto sm:flex-row sm:items-center">
        <Link to="/register">
          <Button className="w-full sm:w-auto">Create an account</Button>
        </Link>
        <Link to="/login">
          <Button variant="secondary" className="w-full sm:w-auto">
            Sign in
          </Button>
        </Link>
      </div>
    </div>
  );
}
