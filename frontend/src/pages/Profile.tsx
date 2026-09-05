import { useEffect, useState } from "react";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api, ApiError } from "../lib/api";
import { useAuth } from "../lib/auth";

export function Profile() {
  const { user, refresh } = useAuth();
  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    if (user) {
      setDisplayName(user.displayName || user.username);
      setBio(user.bio);
    }
  }, [user]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await api("PATCH", "/users/me", { displayName, bio });
      await refresh();
      setMsg("Saved.");
    } catch (err) {
      setMsg(err instanceof ApiError ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  if (!user) return null;

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-semibold tracking-tight">Profile</h1>
      <p className="mt-1 text-sm text-ink-500">
        Your public handle is{" "}
        <span className="break-all font-mono text-ink-900">
          @{user.username}
        </span>.
        Public route:{" "}
        <span className="break-all font-mono">/u/{user.username}</span>{" "}
        (lands in M5).
      </p>
      <Card className="mt-6 p-4 sm:mt-8 sm:p-6">
        <form onSubmit={onSave} className="space-y-4">
          <label className="block">
            <span className="text-xs font-medium text-ink-600">
              Display name
            </span>
            <Input
              className="mt-1"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="text-xs font-medium text-ink-600">Bio</span>
            <Input
              className="mt-1"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
            />
          </label>
          {msg && <p className="text-sm text-ink-500">{msg}</p>}
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
