import { Link } from "react-router-dom";

export function NotFound() {
  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col items-center justify-center px-4 py-8 text-center sm:px-6">
      <p className="text-xs uppercase tracking-[0.2em] text-ink-500">404</p>
      <h1 className="title-gradient mt-2 text-3xl font-semibold tracking-tight">
        Page not found
      </h1>
      <Link
        to="/"
        className="mt-6 text-sm text-ink-700 underline-offset-2 hover:underline"
      >
        Back home
      </Link>
    </div>
  );
}
