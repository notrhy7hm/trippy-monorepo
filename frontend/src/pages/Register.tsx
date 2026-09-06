import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { useAuth } from "../lib/auth";
import { ApiError } from "../lib/api";

const USERNAME_RE = /^[a-z0-9][a-z0-9_-]{2,29}$/;

export function Register() {
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { register } = useAuth();
  const nav = useNavigate();

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const cleanEmail = email.trim();
    const cleanUsername = username.trim().toLowerCase();
    if (!USERNAME_RE.test(cleanUsername)) {
      setError(
        "Username must be 3-30 characters and use lowercase letters, digits, dashes, or underscores.",
      );
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }

    setBusy(true);
    try {
      await register(cleanEmail, cleanUsername, password);
      nav("/app", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Sign-up failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col items-center justify-center px-4 py-8 sm:px-6">
      <Link
        to="/"
        className="mb-8 text-xs uppercase tracking-[0.2em] text-ink-500"
      >
        Trippy.ai
      </Link>
      <Card className="w-full p-5 sm:p-8">
        <h1 className="mb-1 text-2xl font-semibold tracking-tight">
          Create your account
        </h1>
        <p className="mb-6 text-sm text-ink-500">
          You can change everything later.
        </p>
        <form onSubmit={onSubmit} className="space-y-4">
          <label className="block">
            <span className="text-xs font-medium text-ink-600">Email</span>
            <Input
              className="mt-1"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              required
            />
          </label>
          <label className="block">
            <span className="text-xs font-medium text-ink-600">Username</span>
            <Input
              className="mt-1"
              value={username}
              onChange={(e) =>
                setUsername(
                  e.target.value
                    .toLowerCase()
                    .replace(/[^a-z0-9_-]/g, "")
                    .slice(0, 30),
                )
              }
              autoComplete="username"
              required
              minLength={3}
              maxLength={30}
              pattern="[a-z0-9][a-z0-9_-]{2,29}"
            />
            <span className="mt-1 block text-[11px] text-ink-400">
              3-30 characters: lowercase letters, digits, dashes, underscores.
            </span>
          </label>
          <label className="block">
            <span className="text-xs font-medium text-ink-600">Password</span>
            <Input
              className="mt-1"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              required
              minLength={8}
            />
          </label>
          {error && <p className="text-sm text-red-600">{error}</p>}
          <Button type="submit" disabled={busy} className="w-full">
            {busy ? "Creating…" : "Create account"}
          </Button>
        </form>
        <p className="mt-6 text-center text-sm text-ink-500">
          Already have an account?{" "}
          <Link to="/login" className="text-ink-950 hover:underline">
            Sign in
          </Link>
        </p>
      </Card>
    </div>
  );
}
