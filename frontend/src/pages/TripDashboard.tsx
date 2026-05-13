import { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { api, ApiError } from "../lib/api";
import { useAuth } from "../lib/auth";

type Role =
  | "owner"
  | "admin"
  | "planner"
  | "budget_manager"
  | "member"
  | "viewer";

const INVITEABLE_ROLES: Exclude<Role, "owner">[] = [
  "member",
  "admin",
  "planner",
  "budget_manager",
  "viewer",
];

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
  role: Role;
  tags: string[];
};

type Friend = {
  username: string;
  displayName: string;
  since: string;
};

type Party = { username: string; displayName: string };
type Invite = {
  token: string;
  status: string;
  role: Role;
  invitee?: Party;
  invitedBy?: Party;
  expiresAt: string;
  createdAt: string;
};

export function TripDashboard() {
  const { tripSlug } = useParams<{ tripSlug: string }>();
  const { user } = useAuth();

  const [trip, setTrip] = useState<Trip | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState<string | null>(null);

  const [friends, setFriends] = useState<Friend[] | null>(null);
  const [invites, setInvites] = useState<Invite[] | null>(null);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [pickedFriend, setPickedFriend] = useState("");
  const [pickedRole, setPickedRole] = useState<Role>("member");
  const [sending, setSending] = useState(false);
  const [busyToken, setBusyToken] = useState<string | null>(null);

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

  const myMember = members.find((m) => m.username === user?.username);
  const canInvite =
    myMember?.role === "owner" || myMember?.role === "admin";

  const refreshInviteCtx = useCallback(async () => {
    if (!tripSlug) return;
    try {
      const [f, i] = await Promise.all([
        api<Friend[]>("GET", "/friends"),
        api<Invite[]>("GET", `/trips/${tripSlug}/invites`),
      ]);
      setFriends(f);
      setInvites(i);
    } catch (err) {
      setInviteError(
        err instanceof ApiError ? err.message : "Failed to load invites",
      );
      setFriends((prev) => prev ?? []);
      setInvites((prev) => prev ?? []);
    }
  }, [tripSlug]);

  useEffect(() => {
    if (canInvite) refreshInviteCtx();
  }, [canInvite, refreshInviteCtx]);

  async function onSend(e: React.FormEvent) {
    e.preventDefault();
    if (!pickedFriend || !tripSlug) return;
    setSending(true);
    setInviteError(null);
    try {
      await api("POST", `/trips/${tripSlug}/invites`, {
        identifier: pickedFriend,
        role: pickedRole,
      });
      setPickedFriend("");
      setPickedRole("member");
      await refreshInviteCtx();
    } catch (err) {
      setInviteError(
        err instanceof ApiError ? err.message : "Send failed",
      );
    } finally {
      setSending(false);
    }
  }

  async function onRevoke(token: string) {
    if (!tripSlug) return;
    setBusyToken(token);
    setInviteError(null);
    try {
      await api("DELETE", `/trips/${tripSlug}/invites/${token}`);
      await refreshInviteCtx();
    } catch (err) {
      setInviteError(
        err instanceof ApiError ? err.message : "Revoke failed",
      );
    } finally {
      setBusyToken(null);
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

  const memberSet = new Set(members.map((m) => m.username));
  const pendingSet = new Set(
    invites
      ?.map((i) => i.invitee?.username)
      .filter((u): u is string => !!u) ?? [],
  );
  const inviteable =
    friends?.filter(
      (f) => !memberSet.has(f.username) && !pendingSet.has(f.username),
    ) ?? [];

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

      {canInvite ? (
        <div className="mt-10 grid gap-6 md:grid-cols-3">
          <MembersCard className="md:col-span-2 p-6" members={members} />
          <Card className="p-6">
            <h2 className="text-lg font-medium">Invite friends</h2>
            <p className="mt-2 text-sm text-ink-500">
              Only friends can be invited. Add people on the Friends page
              first.
            </p>

            {inviteError && (
              <p className="mt-4 text-sm text-red-600">{inviteError}</p>
            )}

            <form onSubmit={onSend} className="mt-4 space-y-3">
              <label className="block">
                <span className="text-xs font-medium text-ink-600">
                  Friend
                </span>
                {friends === null ? (
                  <p className="mt-1 text-sm text-ink-500">Loading…</p>
                ) : friends.length === 0 ? (
                  <p className="mt-1 text-sm text-ink-500">
                    You have no friends yet — add some on Friends.
                  </p>
                ) : inviteable.length === 0 ? (
                  <p className="mt-1 text-sm text-ink-500">
                    All your friends are already in this trip or invited.
                  </p>
                ) : (
                  <select
                    value={pickedFriend}
                    onChange={(e) => setPickedFriend(e.target.value)}
                    className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
                  >
                    <option value="">Pick a friend…</option>
                    {inviteable.map((f) => (
                      <option key={f.username} value={f.username}>
                        {f.displayName || f.username} (@{f.username})
                      </option>
                    ))}
                  </select>
                )}
              </label>
              <label className="block">
                <span className="text-xs font-medium text-ink-600">Role</span>
                <select
                  value={pickedRole}
                  onChange={(e) => setPickedRole(e.target.value as Role)}
                  className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
                >
                  {INVITEABLE_ROLES.map((r) => (
                    <option key={r} value={r}>
                      {prettyRole(r)}
                    </option>
                  ))}
                </select>
              </label>
              <Button
                type="submit"
                className="w-full"
                disabled={!pickedFriend || sending}
              >
                {sending ? "Sending…" : "Send invite"}
              </Button>
            </form>

            <div className="mt-6">
              <h3 className="text-sm font-medium">Pending invites</h3>
              {invites === null ? (
                <p className="mt-2 text-sm text-ink-500">Loading…</p>
              ) : invites.length === 0 ? (
                <p className="mt-2 text-sm text-ink-500">
                  No invites are currently pending.
                </p>
              ) : (
                <ul className="mt-2 divide-y divide-ink-100">
                  {invites.map((inv) => (
                    <li
                      key={inv.token}
                      className="flex items-center justify-between gap-2 py-2"
                    >
                      <div className="min-w-0">
                        <p className="truncate text-sm">
                          @{inv.invitee?.username ?? "unknown"}
                        </p>
                        <p className="text-xs text-ink-500">
                          {prettyRole(inv.role)} · expires{" "}
                          {new Date(inv.expiresAt).toLocaleDateString()}
                        </p>
                      </div>
                      <Button
                        variant="ghost"
                        onClick={() => onRevoke(inv.token)}
                        disabled={busyToken === inv.token}
                      >
                        Revoke
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </Card>
        </div>
      ) : (
        <div className="mt-10">
          <MembersCard className="p-6" members={members} />
        </div>
      )}
    </div>
  );
}

function MembersCard({
  className,
  members,
}: {
  className: string;
  members: Member[];
}) {
  return (
    <Card className={className}>
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
              {prettyRole(m.role)}
            </span>
          </li>
        ))}
      </ul>
    </Card>
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

function prettyRole(r: string) {
  if (r === "budget_manager") return "Budget manager";
  return r.charAt(0).toUpperCase() + r.slice(1);
}
