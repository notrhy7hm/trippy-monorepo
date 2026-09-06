import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api, ApiError } from "../lib/api";

type Visibility = "private" | "friends" | "public";

export function NewTrip() {
  const nav = useNavigate();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startsOn, setStartsOn] = useState("");
  const [endsOn, setEndsOn] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("private");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (startsOn && endsOn && startsOn > endsOn) {
      setError("Start date must be before or equal to end date.");
      return;
    }

    setBusy(true);
    try {
      const t = await api<{ slug: string }>("POST", "/trips", {
        title,
        description,
        startsOn: startsOn ? new Date(startsOn).toISOString() : undefined,
        endsOn: endsOn ? new Date(endsOn).toISOString() : undefined,
        visibility,
      });
      nav(`/app/trips/${t.slug}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Create failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-semibold tracking-tight">New trip</h1>
      <p className="mt-1 text-sm text-ink-500">
        You can add members and details later.
      </p>
      <Card className="mt-6 p-4 sm:mt-8 sm:p-6">
        <form onSubmit={onSubmit} className="space-y-4">
          <label className="block">
            <span className="text-xs font-medium text-ink-600">Title</span>
            <Input
              className="mt-1"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              placeholder="Summer Italy 2026"
            />
          </label>
          <label className="block">
            <span className="text-xs font-medium text-ink-600">
              Description
            </span>
            <Input
              className="mt-1"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <label className="block">
              <span className="text-xs font-medium text-ink-600">Starts</span>
              <Input
                className="mt-1"
                type="date"
                value={startsOn}
                onChange={(e) => setStartsOn(e.target.value)}
              />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-ink-600">Ends</span>
              <Input
                className="mt-1"
                type="date"
                value={endsOn}
                onChange={(e) => setEndsOn(e.target.value)}
              />
            </label>
          </div>
          <label className="block">
            <span className="text-xs font-medium text-ink-600">
              Visibility
            </span>
            <select
              value={visibility}
              onChange={(e) => setVisibility(e.target.value as Visibility)}
              className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
            >
              <option value="private">Private — only members</option>
              <option value="friends">Friends — your friends can view</option>
              <option value="public">Public — anyone with the link</option>
            </select>
          </label>
          {error && <p className="text-sm text-red-600">{error}</p>}
          <div className="flex flex-col gap-2 sm:flex-row">
            <Button type="submit" disabled={busy} className="w-full sm:w-auto">
              {busy ? "Creating…" : "Create trip"}
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => nav(-1)}
              className="w-full sm:w-auto"
            >
              Cancel
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
