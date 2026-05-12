import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { useAuth } from "../lib/auth";
import { ApiError } from "../lib/api";

export function Login() {
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { login } = useAuth();
  const nav = useNavigate();
  const loc = useLocation();
  const redirect =
    (loc.state as { from?: string } | null)?.from ?? "/app";

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(identifier, password);
      nav(redirect, { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Sign-in failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthFrame title="Welcome back">
      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="text-xs font-medium text-ink-600">
            Email or username
          </span>
          <Input
            className="mt-1"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            autoComplete="username"
            required
          />
        </label>
        <label className="block">
          <span className="text-xs font-medium text-ink-600">Password</span>
          <Input
            className="mt-1"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error && <p className="text-sm text-red-600">{error}</p>}
        <Button type="submit" disabled={busy} className="w-full">
          {busy ? "Signing in…" : "Sign in"}
        </Button>
      </form>
      <p className="mt-6 text-center text-sm text-ink-500">
        New here?{" "}
        <Link to="/register" className="text-ink-950 hover:underline">
          Create an account
        </Link>
      </p>
    </AuthFrame>
  );
}

function AuthFrame({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col items-center justify-center px-6">
      <Link
        to="/"
        className="mb-8 text-xs uppercase tracking-[0.2em] text-ink-500"
      >
        Trippy.ai
      </Link>
      <Card className="w-full p-8">
        <h1 className="mb-6 text-2xl font-semibold tracking-tight">
          {title}
        </h1>
        {children}
      </Card>
    </div>
  );
}
