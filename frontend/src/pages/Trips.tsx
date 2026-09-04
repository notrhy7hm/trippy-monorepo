import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api } from "../lib/api";

type Trip = {
  slug: string;
  title: string;
  description: string;
  startsOn?: string;
  endsOn?: string;
  visibility: "private" | "friends" | "public";
};

export function Trips() {
  const [trips, setTrips] = useState<Trip[] | null>(null);

  useEffect(() => {
    api<Trip[]>("GET", "/trips").then(setTrips).catch(() => setTrips([]));
  }, []);

  return (
    <div>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Your trips</h1>
          <p className="mt-1 text-sm text-ink-500">
            Owned and joined.
          </p>
        </div>
        <Link to="/app/trips/new" className="w-full sm:w-auto">
          <Button className="w-full sm:w-auto">New trip</Button>
        </Link>
      </div>

      <div className="mt-6 grid gap-3 sm:mt-8">
        {trips === null && (
          <p className="text-sm text-ink-500">Loading…</p>
        )}
        {trips && trips.length === 0 && (
          <Card className="p-8 text-center">
            <p className="text-sm text-ink-500">
              No trips yet. Create your first one.
            </p>
          </Card>
        )}
        {trips?.map((t) => (
          <Link key={t.slug} to={`/app/trips/${t.slug}`}>
            <Card className="p-4 transition-colors hover:border-ink-400 sm:p-5">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <h2 className="text-lg font-medium">{t.title}</h2>
                  <p className="mt-1 text-sm text-ink-500">
                    {formatRange(t.startsOn, t.endsOn)} ·{" "}
                    <span className="break-all font-mono text-xs">{t.slug}</span>
                  </p>
                </div>
                <span className="w-fit rounded border border-ink-200 px-2 py-0.5 text-xs uppercase tracking-wider text-ink-500">
                  {t.visibility}
                </span>
              </div>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}

function formatRange(a?: string, b?: string) {
  if (!a && !b) return "No dates set";
  const fmt = (d: string) => new Date(d).toLocaleDateString();
  if (a && b) return `${fmt(a)} – ${fmt(b)}`;
  return fmt(a ?? b!);
}
