import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { api, ApiError } from "../lib/api";

type Friend = {
  username: string;
  displayName: string;
  since: string;
};

type Request = {
  username: string;
  displayName: string;
  message?: string;
  createdAt: string;
};

type Relation =
  | "none"
  | "self"
  | "friend"
  | "incoming_request"
  | "outgoing_request";

type SearchResult = {
  username: string;
  displayName: string;
  avatarUrl?: string;
  relation: Relation;
};

export function Friends() {
  const [friends, setFriends] = useState<Friend[] | null>(null);
  const [incoming, setIncoming] = useState<Request[] | null>(null);
  const [outgoing, setOutgoing] = useState<Request[] | null>(null);
  const [q, setQ] = useState("");
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [busyUser, setBusyUser] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const [f, inc, out] = await Promise.all([
        api<Friend[]>("GET", "/friends"),
        api<Request[]>("GET", "/friend-requests/incoming"),
        api<Request[]>("GET", "/friend-requests/outgoing"),
      ]);
      setFriends(f);
      setIncoming(inc);
      setOutgoing(out);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load");
      setFriends((prev) => prev ?? []);
      setIncoming((prev) => prev ?? []);
      setOutgoing((prev) => prev ?? []);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Debounced search — fires 250ms after the user stops typing.
  useEffect(() => {
    const term = q.trim();
    if (term.length === 0) {
      setResults(null);
      setSearching(false);
      return;
    }
    setSearching(true);
    const id = window.setTimeout(async () => {
      try {
        const r = await api<SearchResult[]>(
          "GET",
          `/users/search?q=${encodeURIComponent(term)}`,
        );
        setResults(r);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "Search failed");
        setResults([]);
      } finally {
        setSearching(false);
      }
    }, 250);
    return () => {
      window.clearTimeout(id);
    };
  }, [q]);

  const patchResultRelation = useCallback(
    (username: string, next: Relation) => {
      setResults((prev) =>
        prev
          ? prev.map((r) =>
              r.username === username ? { ...r, relation: next } : r,
            )
          : prev,
      );
    },
    [],
  );

  async function withBusy(username: string, fn: () => Promise<void>) {
    setError(null);
    setBusyUser(username);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Action failed");
    } finally {
      setBusyUser(null);
    }
  }

  async function sendRequest(username: string) {
    await withBusy(username, async () => {
      await api("POST", "/friend-requests", { username });
      patchResultRelation(username, "outgoing_request");
      await refresh();
    });
  }

  async function acceptRequest(username: string) {
    await withBusy(username, async () => {
      await api("POST", `/friend-requests/from/${username}/accept`);
      patchResultRelation(username, "friend");
      await refresh();
    });
  }

  async function declineRequest(username: string) {
    await withBusy(username, async () => {
      await api("POST", `/friend-requests/from/${username}/decline`);
      patchResultRelation(username, "none");
      await refresh();
    });
  }

  async function cancelRequest(username: string) {
    await withBusy(username, async () => {
      await api("DELETE", `/friend-requests/to/${username}`);
      patchResultRelation(username, "none");
      await refresh();
    });
  }

  async function removeFriend(username: string) {
    if (!window.confirm(`Remove @${username} from your friends?`)) return;
    await withBusy(username, async () => {
      await api("DELETE", `/friends/${username}`);
      patchResultRelation(username, "none");
      await refresh();
    });
  }

  return (
    <div className="space-y-10">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight">Friends</h1>
        <p className="mt-1 text-sm text-ink-500">
          Find people, send requests, manage your list.
        </p>
      </header>

      {error && (
        <Card className="border-red-200 bg-red-50/40 p-4">
          <p className="text-sm text-red-700">{error}</p>
        </Card>
      )}

      <Section
        title="Find people"
        hint="Search by username or display name."
      >
        <Input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="alice, Alice Smith…"
          autoComplete="off"
        />
        <ResultsList
          q={q}
          searching={searching}
          results={results}
          busyUser={busyUser}
          onSend={sendRequest}
          onAccept={acceptRequest}
          onCancel={cancelRequest}
          onRemove={removeFriend}
        />
      </Section>

      <Section
        title="Incoming requests"
        hint={
          incoming && incoming.length > 0
            ? `${incoming.length} pending.`
            : undefined
        }
      >
        {incoming === null ? (
          <Loading />
        ) : incoming.length === 0 ? (
          <Empty>No incoming requests.</Empty>
        ) : (
          <ul className="divide-y divide-ink-100">
            {incoming.map((req) => (
              <li
                key={req.username}
                className="flex items-center justify-between py-3"
              >
                <PersonBlock
                  username={req.username}
                  displayName={req.displayName}
                  hint={req.message}
                />
                <div className="flex gap-2">
                  <Button
                    onClick={() => acceptRequest(req.username)}
                    disabled={busyUser === req.username}
                  >
                    Accept
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => declineRequest(req.username)}
                    disabled={busyUser === req.username}
                  >
                    Decline
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section
        title="Outgoing requests"
        hint={
          outgoing && outgoing.length > 0
            ? `${outgoing.length} pending.`
            : undefined
        }
      >
        {outgoing === null ? (
          <Loading />
        ) : outgoing.length === 0 ? (
          <Empty>You have no pending requests.</Empty>
        ) : (
          <ul className="divide-y divide-ink-100">
            {outgoing.map((req) => (
              <li
                key={req.username}
                className="flex items-center justify-between py-3"
              >
                <PersonBlock
                  username={req.username}
                  displayName={req.displayName}
                  hint={req.message}
                />
                <Button
                  variant="ghost"
                  onClick={() => cancelRequest(req.username)}
                  disabled={busyUser === req.username}
                >
                  Cancel
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section
        title="Your friends"
        hint={
          friends && friends.length > 0 ? `${friends.length} total.` : undefined
        }
      >
        {friends === null ? (
          <Loading />
        ) : friends.length === 0 ? (
          <Empty>No friends yet — find someone above.</Empty>
        ) : (
          <ul className="divide-y divide-ink-100">
            {friends.map((f) => (
              <li
                key={f.username}
                className="flex items-center justify-between py-3"
              >
                <PersonBlock
                  username={f.username}
                  displayName={f.displayName}
                  hint={`Friends since ${new Date(f.since).toLocaleDateString()}`}
                />
                <Button
                  variant="ghost"
                  onClick={() => removeFriend(f.username)}
                  disabled={busyUser === f.username}
                >
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}

function Section({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <section>
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-lg font-medium">{title}</h2>
        {hint && <p className="text-xs text-ink-500">{hint}</p>}
      </div>
      <Card className="p-5">{children}</Card>
    </section>
  );
}

function PersonBlock({
  username,
  displayName,
  hint,
}: {
  username: string;
  displayName: string;
  hint?: string;
}) {
  return (
    <div className="min-w-0">
      <p className="truncate text-sm font-medium">{displayName || username}</p>
      <p className="truncate text-xs text-ink-500">
        @{username}
        {hint ? <> &middot; <span className="italic">{hint}</span></> : null}
      </p>
    </div>
  );
}

function Loading() {
  return <p className="py-3 text-sm text-ink-500">Loading…</p>;
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="py-3 text-sm text-ink-500">{children}</p>;
}

function ResultsList({
  q,
  searching,
  results,
  busyUser,
  onSend,
  onAccept,
  onCancel,
  onRemove,
}: {
  q: string;
  searching: boolean;
  results: SearchResult[] | null;
  busyUser: string | null;
  onSend: (u: string) => void;
  onAccept: (u: string) => void;
  onCancel: (u: string) => void;
  onRemove: (u: string) => void;
}) {
  if (q.trim().length === 0) return null;
  if (results === null) {
    return (
      <p className="mt-4 text-sm text-ink-500">
        {searching ? "Searching…" : ""}
      </p>
    );
  }
  if (results.length === 0) {
    return <p className="mt-4 text-sm text-ink-500">No matches.</p>;
  }
  return (
    <ul className="mt-4 divide-y divide-ink-100">
      {results.map((r) => (
        <li
          key={r.username}
          className="flex items-center justify-between py-3"
        >
          <PersonBlock username={r.username} displayName={r.displayName} />
          <ResultAction
            relation={r.relation}
            username={r.username}
            busy={busyUser === r.username}
            onSend={onSend}
            onAccept={onAccept}
            onCancel={onCancel}
            onRemove={onRemove}
          />
        </li>
      ))}
    </ul>
  );
}

function ResultAction({
  relation,
  username,
  busy,
  onSend,
  onAccept,
  onCancel,
  onRemove,
}: {
  relation: Relation;
  username: string;
  busy: boolean;
  onSend: (u: string) => void;
  onAccept: (u: string) => void;
  onCancel: (u: string) => void;
  onRemove: (u: string) => void;
}) {
  switch (relation) {
    case "self":
      return <Badge>You</Badge>;
    case "friend":
      return (
        <div className="flex items-center gap-2">
          <Badge>Friend</Badge>
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => onRemove(username)}
          >
            Remove
          </Button>
        </div>
      );
    case "incoming_request":
      return (
        <Button disabled={busy} onClick={() => onAccept(username)}>
          Accept
        </Button>
      );
    case "outgoing_request":
      return (
        <Button
          variant="secondary"
          disabled={busy}
          onClick={() => onCancel(username)}
        >
          Cancel
        </Button>
      );
    case "none":
    default:
      return (
        <Button disabled={busy} onClick={() => onSend(username)}>
          Add friend
        </Button>
      );
  }
}

function Badge({ children }: { children: ReactNode }) {
  return (
    <span className="rounded border border-ink-200 px-2 py-0.5 text-xs uppercase tracking-wider text-ink-500">
      {children}
    </span>
  );
}
