import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api, ApiError } from "../lib/api";
import { useAuth } from "../lib/auth";

type Party = { username: string; displayName: string };
type InviteTrip = { slug: string; title: string };
type TripInvite = {
  token: string;
  role: string;
  trip?: InviteTrip;
  invitedBy?: Party;
  expiresAt: string;
  createdAt: string;
};

type AcceptResponse = {
  slug?: string;
};

export function AppHome() {
  const { user } = useAuth();
  const nav = useNavigate();

  const [invites, setInvites] = useState<TripInvite[] | null>(null);
  const [busyToken, setBusyToken] = useState<string | null>(null);
  // loadError is shown subtly when the invites list itself cannot load.
  const [loadError, setLoadError] = useState<string | null>(null);
  // actionError is shown inside the invites Card when accept/decline fails.
  const [actionError, setActionError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const xs = await api<TripInvite[]>("GET", "/me/trip-invites");
      setInvites(xs);
      setLoadError(null);
    } catch (err) {
      setInvites((prev) => prev ?? []);
      setLoadError(
        err instanceof ApiError ? err.message : "Failed to load invites",
      );
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function accept(inv: TripInvite) {
    setActionError(null);
    setBusyToken(inv.token);
    try {
      // Backend returns the joined Trip on accept; prefer its slug because
      // it reflects the actual trip the user just joined. Fall back to the
      // invite's snapshot only if the response somehow omits it.
      const resp = await api<AcceptResponse>(
        "POST",
        `/invites/${inv.token}/accept`,
      );
      const slug = resp?.slug ?? inv.trip?.slug;
      if (slug) {
        nav(`/app/trips/${slug}`);
        return;
      }
      await refresh();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : "Accept failed");
    } finally {
      setBusyToken(null);
    }
  }

  async function decline(inv: TripInvite) {
    setActionError(null);
    setBusyToken(inv.token);
    try {
      await api("POST", `/invites/${inv.token}/decline`);
      await refresh();
    } catch (err) {
      setActionError(err instanceof ApiError ? err.message : "Decline failed");
    } finally {
      setBusyToken(null);
    }
  }

  const hasInvites = (invites?.length ?? 0) > 0;

  return (
    <div>
      <h1 className="title-gradient text-3xl font-semibold tracking-tight">
        Hello, {user?.displayName || user?.username}.
      </h1>
      <p className="mt-2 text-ink-500">Pick a trip, or start a new one.</p>

      {!hasInvites && loadError && (
        <p className="mt-4 text-xs text-ink-500">
          Trip invites are temporarily unavailable.
        </p>
      )}

      {hasInvites && invites && (
        <Card className="mt-8 p-6">
          <h2 className="text-lg font-medium">Trip invites</h2>
          <p className="mt-1 text-sm text-ink-500">
            People have invited you to plan trips together.
          </p>
          {actionError && (
            <p className="mt-3 text-sm text-red-600">{actionError}</p>
          )}
          <ul className="mt-4 divide-y divide-ink-100">
            {invites.map((inv) => (
              <li
                key={inv.token}
                className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="min-w-0">
                  <p className="text-sm font-medium">
                    {inv.trip?.title ?? "Untitled trip"}
                  </p>
                  <p className="text-xs text-ink-500">
                    @{inv.invitedBy?.username ?? "unknown"} invited you as{" "}
                    {prettyRole(inv.role)} · expires{" "}
                    {new Date(inv.expiresAt).toLocaleDateString()}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button
                    onClick={() => accept(inv)}
                    disabled={busyToken === inv.token}
                  >
                    Accept
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => decline(inv)}
                    disabled={busyToken === inv.token}
                  >
                    Decline
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <div className="mt-10 grid gap-4 md:grid-cols-3">
        <Link to="/app/trips/new">
          <Card className="h-full p-6 transition-colors hover:border-ink-400">
            <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
              New
            </p>
            <h2 className="mt-2 text-lg font-medium">Create a trip</h2>
            <p className="mt-2 text-sm text-ink-500">
              Title, dates, visibility. Invite friends after.
            </p>
          </Card>
        </Link>
        <Link to="/app/trips">
          <Card className="h-full p-6 transition-colors hover:border-ink-400">
            <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
              Browse
            </p>
            <h2 className="mt-2 text-lg font-medium">Your trips</h2>
            <p className="mt-2 text-sm text-ink-500">
              Past and upcoming, all in one place.
            </p>
          </Card>
        </Link>
        <Link to="/app/friends">
          <Card className="h-full p-6 transition-colors hover:border-ink-400">
            <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
              Connect
            </p>
            <h2 className="mt-2 text-lg font-medium">Friends</h2>
            <p className="mt-2 text-sm text-ink-500">
              Search people and manage your requests.
            </p>
          </Card>
        </Link>
      </div>
    </div>
  );
}

function prettyRole(r: string) {
  if (r === "budget_manager") return "Budget manager";
  return r.charAt(0).toUpperCase() + r.slice(1);
}
