import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api, ApiError } from "../lib/api";

type Trip = {
  slug: string;
  title: string;
  description: string;
  startsOn?: string;
  endsOn?: string;
  visibility: string;
  createdAt: string;
};

type Member = {
  username: string;
  displayName: string;
  role: "owner" | "admin" | "member";
  tags: string[];
};

export function TripDashboard() {
  const { tripSlug } = useParams<{ tripSlug: string }>();
  const [trip, setTrip] = useState<Trip | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [inviteMsg, setInviteMsg] = useState<string | null>(null);

  useEffect(() => {
    if (!tripSlug) return;
    Promise.all([
      api<Trip>("GET", `/trips/${tripSlug}`),
      api<Member[]>("GET", `/trips/${tripSlug}/members`),
    ])
      .then(([t, m]) => {
        setTrip(t);
        setMembers(m);
      })
      .catch((err) =>
        setError(err instanceof ApiError ? err.message : "Failed to load"),
      );
  }, [tripSlug]);

  async function onInvite() {
    if (!tripSlug) return;
    setInviteMsg(null);
    try {
      const res = await api<{ message: string }>(
        "POST",
        `/trips/${tripSlug}/invites`,
        {},
      );
      setInviteMsg(res.message ?? "OK");
    } catch (err) {
      setInviteMsg(err instanceof ApiError ? err.message : "Invite failed");
    }
  }

  if (error) {
    return (
      <Card className="p-8 text-center">
        <p className="text-sm text-ink-500">{error}</p>
      </Card>
    );
  }
  if (!trip) {
    return <p className="text-sm text-ink-500">Loading…</p>;
  }

  return (
    <div>
      <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
        Trip · {trip.visibility}
      </p>
      <h1 className="title-gradient mt-1 text-4xl font-semibold tracking-tight">
        {trip.title}
      </h1>
      {trip.description && (
        <p className="mt-3 max-w-2xl text-ink-600">{trip.description}</p>
      )}
      <p className="mt-2 text-sm text-ink-500">
        <span className="font-mono">{trip.slug}</span>
      </p>

      <div className="mt-10 grid gap-4 md:grid-cols-3">
        <PlannerCard title="Tasks" hint="Kanban board · M2" />
        <PlannerCard title="Timeline" hint="Branching timeline · M2" />
        <PlannerCard title="Budget" hint="Envelopes & splits · M2" />
      </div>

      <div className="mt-10 grid gap-6 md:grid-cols-3">
        <Card className="md:col-span-2 p-6">
          <h2 className="text-lg font-medium">Members</h2>
          <ul className="mt-4 divide-y divide-ink-100">
            {members.map((m) => (
              <li
                key={m.username}
                className="flex items-center justify-between py-3"
              >
                <div>
                  <p className="text-sm font-medium">{m.displayName}</p>
                  <p className="text-xs text-ink-500">@{m.username}</p>
                </div>
                <span className="rounded border border-ink-200 px-2 py-0.5 text-xs uppercase tracking-wider text-ink-500">
                  {m.role}
                </span>
              </li>
            ))}
          </ul>
        </Card>
        <Card className="p-6">
          <h2 className="text-lg font-medium">Invite</h2>
          <p className="mt-2 text-sm text-ink-500">
            Full invites land in M1. The route is wired now so the UI is
            stable.
          </p>
          <Button onClick={onInvite} className="mt-4 w-full">
            Send placeholder invite
          </Button>
          {inviteMsg && (
            <p className="mt-3 text-xs text-ink-500">{inviteMsg}</p>
          )}
        </Card>
      </div>
    </div>
  );
}

function PlannerCard({ title, hint }: { title: string; hint: string }) {
  return (
    <Card className="p-6 opacity-70">
      <p className="text-xs uppercase tracking-[0.18em] text-ink-500">Soon</p>
      <h3 className="mt-2 text-lg font-medium">{title}</h3>
      <p className="mt-2 text-sm text-ink-500">{hint}</p>
    </Card>
  );
}
