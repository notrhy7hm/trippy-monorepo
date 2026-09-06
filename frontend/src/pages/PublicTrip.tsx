import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api, ApiError } from "../lib/api";

type PublicTripResponse = {
  slug: string;
  title: string;
  description: string;
  startsOn?: string;
  endsOn?: string;
  visibility: "public";
};

export function PublicTrip() {
  const { tripSlug } = useParams<{ tripSlug: string }>();
  const [trip, setTrip] = useState<PublicTripResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setTrip(null);
    setError(null);

    if (!tripSlug) {
      setError("Trip not found.");
      return;
    }

    api<PublicTripResponse>(
      "GET",
      `/public/trips/${encodeURIComponent(tripSlug)}`,
    )
      .then((t) => {
        if (!active) return;
        setTrip(t);
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof ApiError ? err.message : "Trip not found.");
      });

    return () => {
      active = false;
    };
  }, [tripSlug]);

  if (error) {
    return (
      <div className="mx-auto flex min-h-screen max-w-2xl items-center px-4 py-10">
        <Card className="w-full p-8 text-center">
          <p className="text-sm text-ink-500">{error}</p>
          <div className="mt-6">
            <Link to="/">
              <Button variant="secondary">Back home</Button>
            </Link>
          </div>
        </Card>
      </div>
    );
  }

  if (!trip) {
    return (
      <div className="mx-auto flex min-h-screen max-w-2xl items-center px-4 py-10">
        <p className="text-sm text-ink-500">Loading…</p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-3xl items-center px-4 py-10">
      <Card className="w-full p-6 sm:p-8">
        <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
          Public trip
        </p>
        <h1 className="title-gradient mt-2 break-words text-3xl font-semibold tracking-tight sm:text-5xl">
          {trip.title}
        </h1>
        {trip.description && (
          <p className="mt-4 max-w-2xl whitespace-pre-line text-ink-600">
            {trip.description}
          </p>
        )}
        <p className="mt-4 text-sm text-ink-500">
          {formatRange(trip.startsOn, trip.endsOn)}
        </p>
        <p className="mt-2 break-all font-mono text-xs text-ink-400">
          {trip.slug}
        </p>
        <div className="mt-8 flex flex-col gap-3 sm:flex-row">
          <Link to="/register" className="w-full sm:w-auto">
            <Button className="w-full sm:w-auto">Create an account</Button>
          </Link>
          <Link to="/login" className="w-full sm:w-auto">
            <Button variant="secondary" className="w-full sm:w-auto">
              Sign in
            </Button>
          </Link>
        </div>
      </Card>
    </div>
  );
}

function formatRange(a?: string, b?: string) {
  if (!a && !b) return "No dates set";
  const fmt = (d: string) => d.match(/^(\d{4}-\d{2}-\d{2})/)?.[1] ?? d;
  if (a && b) return `${fmt(a)} – ${fmt(b)}`;
  return fmt(a ?? b!);
}
