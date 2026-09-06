import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { KeyboardEvent } from "react";
import { useParams } from "react-router-dom";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { Input } from "../components/ui/Input";
import { PlanningBoard } from "../components/PlanningBoard";
import { ItineraryBoard } from "../components/ItineraryBoard";
import { BudgetBoard } from "../components/BudgetBoard";
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

const MAX_TAGS = 8;
const MAX_TAG_LEN = 24;
const TAG_PATTERN = /^[a-z0-9_-]+$/;

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
  userId: string;
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

type RoleEditingProps = {
  expandedMember: string | null;
  onOpen: (username: string) => void;
  onChange: (username: string, role: Role) => void;
};

type TagsEditingProps = {
  editingMember: string | null;
  draft: string[];
  input: string;
  setInput: (v: string) => void;
  onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
  onRemove: (t: string) => void;
  onSave: (username: string) => void;
  onOpen: (username: string, current: string[]) => void;
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
  const [friendQuery, setFriendQuery] = useState("");
  const [pickedRole, setPickedRole] = useState<Role>("member");
  const [sending, setSending] = useState(false);
  const [busyToken, setBusyToken] = useState<string | null>(null);

  // One editor open across the whole Members card at a time. Switching
  // editors is centralized in openRoleEditor / openTagsEditor /
  // closeEditors so role and tags can never both be open on the same row.
  const [expandedMember, setExpandedMember] = useState<string | null>(null);
  const [tagsEditingMember, setTagsEditingMember] = useState<string | null>(
    null,
  );
  const [tagDraft, setTagDraft] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");

  const [busyMember, setBusyMember] = useState<string | null>(null);
  const [memberError, setMemberError] = useState<string | null>(null);

  const genRef = useRef(0);

  const loadTripAndMembers = useCallback(
    async (gen: number) => {
      if (!tripSlug) return;
      try {
        const [t, m] = await Promise.all([
          api<Trip>("GET", `/trips/${tripSlug}`),
          api<Member[]>("GET", `/trips/${tripSlug}/members`),
        ]);
        if (gen !== genRef.current) return;
        setTrip(t);
        setMembers(m);
        setError(null);
      } catch (err) {
        if (gen !== genRef.current) return;
        setError(err instanceof ApiError ? err.message : "Failed to load");
      }
    },
    [tripSlug],
  );

  const reloadTrip = useCallback(
    () => loadTripAndMembers(genRef.current),
    [loadTripAndMembers],
  );

  useEffect(() => {
    const gen = ++genRef.current;
    setError(null);
    setTrip(null);
    setMembers([]);
    setFriends(null);
    setInvites(null);
    setInviteError(null);
    setPickedFriend("");
    setFriendQuery("");
    setPickedRole("member");
    setSending(false);
    setBusyToken(null);
    setExpandedMember(null);
    setTagsEditingMember(null);
    setTagDraft([]);
    setTagInput("");
    setBusyMember(null);
    setMemberError(null);

    loadTripAndMembers(gen);
  }, [tripSlug, loadTripAndMembers]);

  const myMember = members.find((m) => m.username === user?.username);
  const canInvite =
    myMember?.role === "owner" || myMember?.role === "admin";
  const canManagePlanning =
    myMember?.role === "owner" ||
    myMember?.role === "admin" ||
    myMember?.role === "planner";
  const canManageBudget =
    myMember?.role === "owner" ||
    myMember?.role === "admin" ||
    myMember?.role === "budget_manager";

  const refreshInviteCtx = useCallback(async () => {
    if (!tripSlug) return;
    const gen = genRef.current;
    try {
      const [f, i] = await Promise.all([
        api<Friend[]>("GET", "/friends"),
        api<Invite[]>("GET", `/trips/${tripSlug}/invites`),
      ]);
      if (gen !== genRef.current) return;
      setFriends(f);
      setInvites(i);
    } catch (err) {
      if (gen !== genRef.current) return;
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

  const memberSet = useMemo(
    () => new Set(members.map((m) => m.username)),
    [members],
  );
  const pendingSet = useMemo(
    () =>
      new Set(
        invites
          ?.map((i) => i.invitee?.username)
          .filter((u): u is string => !!u) ?? [],
      ),
    [invites],
  );
  const inviteable = useMemo(
    () =>
      friends?.filter(
        (f) => !memberSet.has(f.username) && !pendingSet.has(f.username),
      ) ?? [],
    [friends, memberSet, pendingSet],
  );

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
      setFriendQuery("");
      setPickedRole("member");
      await refreshInviteCtx();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setInviteError(
          "You don't have permission to invite people to this trip anymore.",
        );
        await reloadTrip();
      } else {
        setInviteError(err instanceof ApiError ? err.message : "Send failed");
      }
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
      if (err instanceof ApiError && err.status === 403) {
        setInviteError(
          "You don't have permission to revoke invites in this trip anymore.",
        );
        await reloadTrip();
      } else {
        setInviteError(err instanceof ApiError ? err.message : "Revoke failed");
      }
    } finally {
      setBusyToken(null);
    }
  }

  // ---------------------------------------------------------------------
  // role + tag editor wiring
  // ---------------------------------------------------------------------

  function openRoleEditor(username: string) {
    setMemberError(null);
    setTagsEditingMember(null);
    setTagDraft([]);
    setTagInput("");
    setExpandedMember(username);
  }

  function openTagsEditor(username: string, current: string[]) {
    setMemberError(null);
    setExpandedMember(null);
    setTagsEditingMember(username);
    setTagDraft(current);
    setTagInput("");
  }

  // closeEditors collapses any open role/tag editor row and (by default)
  // also clears the local memberError. Pass { clearError: false } when a
  // caller has just set a member error that must remain visible after the
  // editor closes (e.g. a 403/404 from PATCH /tags where reloadTrip() also
  // ran).
  function closeEditors(opts: { clearError?: boolean } = {}) {
    const { clearError = true } = opts;
    setExpandedMember(null);
    setTagsEditingMember(null);
    setTagDraft([]);
    setTagInput("");
    if (clearError) {
      setMemberError(null);
    }
  }

  function onRemoveTag(t: string) {
    setTagDraft((prev) => prev.filter((x) => x !== t));
  }

  function commitTagFromInput() {
    if (tagInput.trim() === "") {
      setTagInput("");
      return;
    }
    const result = appendTag(tagDraft, tagInput);
    if (result.error) {
      setMemberError(result.error);
      return;
    }
    setTagDraft(result.tags);
    setTagInput("");
    setMemberError(null);
  }

  function onTagInputKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commitTagFromInput();
    } else if (
      e.key === "Backspace" &&
      tagInput === "" &&
      tagDraft.length > 0
    ) {
      e.preventDefault();
      setTagDraft((prev) => prev.slice(0, -1));
    }
  }

  async function onChangeRole(username: string, newRole: Role) {
    if (!tripSlug || busyMember !== null) return;
    const gen = genRef.current;
    setBusyMember(username);
    setMemberError(null);
    try {
      const updated = await api<Member>(
        "PATCH",
        `/trips/${tripSlug}/members/${encodeURIComponent(username)}/role`,
        { role: newRole },
      );
      if (gen !== genRef.current) return;
      setMembers((prev) =>
        prev.map((m) =>
          m.username === username ? { ...m, ...updated } : m,
        ),
      );
      setExpandedMember(null);
    } catch (err) {
      if (gen !== genRef.current) return;
      if (err instanceof ApiError && err.status === 403) {
        setMemberError(
          "You do not have permission to manage roles in this trip.",
        );
      } else {
        setMemberError(
          err instanceof ApiError ? err.message : "Role change failed",
        );
      }
      if (
        err instanceof ApiError &&
        (err.status === 400 || err.status === 403 || err.status === 404)
      ) {
        await reloadTrip();
        if (gen !== genRef.current) return;
      }
      setExpandedMember(null);
    } finally {
      if (gen === genRef.current) {
        setBusyMember(null);
      }
    }
  }

  async function onSaveTags(username: string) {
    if (!tripSlug || busyMember !== null) return;
    setMemberError(null);

    // Auto-commit any trailing input the user typed but didn't Enter.
    let finalTags = tagDraft;
    if (tagInput.trim() !== "") {
      const result = appendTag(tagDraft, tagInput);
      if (result.error) {
        setMemberError(result.error);
        return;
      }
      finalTags = result.tags;
    }

    const gen = genRef.current;
    setBusyMember(username);
    try {
      const updated = await api<Member>(
        "PATCH",
        `/trips/${tripSlug}/members/${encodeURIComponent(username)}/tags`,
        { tags: finalTags },
      );
      if (gen !== genRef.current) return;
      setMembers((prev) =>
        prev.map((m) =>
          m.username === username ? { ...m, ...updated } : m,
        ),
      );
      closeEditors();
    } catch (err) {
      if (gen !== genRef.current) return;
      if (err instanceof ApiError && err.status === 403) {
        setMemberError(
          "You do not have permission to manage tags in this trip.",
        );
      } else {
        setMemberError(
          err instanceof ApiError ? err.message : "Save failed",
        );
      }
      // 403 / 404 typically mean stale state (permissions drifted, member
      // removed) — refresh and close the editor. 400 means the backend
      // disagreed with the payload; keep the editor open so the user can
      // fix it. Pass clearError:false so the memberError we just set
      // survives closeEditors().
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await reloadTrip();
        if (gen !== genRef.current) return;
        closeEditors({ clearError: false });
      }
    } finally {
      if (gen === genRef.current) {
        setBusyMember(null);
      }
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

  const roleEditor: RoleEditingProps = {
    expandedMember,
    onOpen: openRoleEditor,
    onChange: onChangeRole,
  };
  const tagsEditor: TagsEditingProps = {
    editingMember: tagsEditingMember,
    draft: tagDraft,
    input: tagInput,
    setInput: setTagInput,
    onKeyDown: onTagInputKeyDown,
    onRemove: onRemoveTag,
    onSave: onSaveTags,
    onOpen: openTagsEditor,
  };

  return (
    <div className="min-w-0">
      <p className="text-xs uppercase tracking-[0.18em] text-ink-500">
        Trip · {trip.visibility}
      </p>
      <h1 className="title-gradient mt-1 break-words text-3xl font-semibold tracking-tight sm:text-4xl">
        {trip.title}
      </h1>
      {trip.description && (
        <p className="mt-3 max-w-2xl text-ink-600">{trip.description}</p>
      )}
      <p className="mt-2 text-sm text-ink-500">
        <span className="break-all font-mono">{trip.slug}</span>
      </p>

      <PlanningBoard
        tripSlug={tripSlug}
        members={members}
        viewerIsMember={!!myMember}
        canManage={canManagePlanning}
        reloadTrip={reloadTrip}
      />

      <ItineraryBoard
        tripSlug={tripSlug}
        members={members}
        viewerIsMember={!!myMember}
        canManage={canManagePlanning}
        reloadTrip={reloadTrip}
        tripStartsOn={trip.startsOn}
        tripEndsOn={trip.endsOn}
      />

      <BudgetBoard
        tripSlug={tripSlug}
        members={members}
        viewerIsMember={!!myMember}
        canManage={canManageBudget}
        currentUserId={myMember?.userId}
        reloadTrip={reloadTrip}
      />

      <div className="mt-8 grid gap-4 sm:mt-10 md:grid-cols-2">
        <PlannerCard title="Timeline" hint="Branching timeline · later" />
      </div>

      {canInvite ? (
        <div className="mt-10 grid gap-6 md:grid-cols-3">
          <MembersCard
            className="p-4 sm:p-6 md:col-span-2"
            members={members}
            viewerIsOwner={myMember?.role === "owner"}
            busyMember={busyMember}
            memberError={memberError}
            onCancelEdit={closeEditors}
            role={roleEditor}
            tags={tagsEditor}
          />
          <Card className="p-4 sm:p-6">
            <h2 className="text-lg font-medium">Invite friends</h2>
            <p className="mt-2 text-sm text-ink-500">
              Only friends can be invited. Add people on the Friends page
              first.
            </p>

            {inviteError && (
              <p className="mt-4 text-sm text-red-600">{inviteError}</p>
            )}

            <form onSubmit={onSend} className="mt-4 space-y-4">
              <div>
                <span className="text-xs font-medium text-ink-600">
                  Friend
                </span>
                <FriendPicker
                  className="mt-1"
                  friends={friends}
                  inviteable={inviteable}
                  query={friendQuery}
                  setQuery={setFriendQuery}
                  picked={pickedFriend}
                  setPicked={setPickedFriend}
                />
              </div>
              <div>
                <span className="text-xs font-medium text-ink-600">Role</span>
                <RolePills
                  className="mt-2"
                  picked={pickedRole}
                  setPicked={setPickedRole}
                />
              </div>
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
                      className="flex flex-col gap-2 py-2 sm:flex-row sm:items-center sm:justify-between"
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
                        className="w-full sm:w-auto"
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
          <MembersCard
            className="p-4 sm:p-6"
            members={members}
            viewerIsOwner={myMember?.role === "owner"}
            busyMember={busyMember}
            memberError={memberError}
            onCancelEdit={closeEditors}
            role={roleEditor}
            tags={tagsEditor}
          />
        </div>
      )}
    </div>
  );
}

function FriendPicker({
  className,
  friends,
  inviteable,
  query,
  setQuery,
  picked,
  setPicked,
}: {
  className?: string;
  friends: Friend[] | null;
  inviteable: Friend[];
  query: string;
  setQuery: (s: string) => void;
  picked: string;
  setPicked: (u: string) => void;
}) {
  if (friends === null) {
    return (
      <p className={[className ?? "", "text-sm text-ink-500"].join(" ")}>
        Loading…
      </p>
    );
  }
  if (friends.length === 0) {
    return (
      <p className={[className ?? "", "text-sm text-ink-500"].join(" ")}>
        You have no friends yet — add some on Friends.
      </p>
    );
  }
  if (inviteable.length === 0) {
    return (
      <p className={[className ?? "", "text-sm text-ink-500"].join(" ")}>
        All your friends are already in this trip or invited.
      </p>
    );
  }

  const norm = query.trim().toLowerCase();
  const filtered =
    norm.length === 0
      ? inviteable
      : inviteable.filter(
          (f) =>
            f.username.toLowerCase().includes(norm) ||
            f.displayName.toLowerCase().includes(norm),
        );

  return (
    <div className={[className ?? "", "space-y-2"].join(" ")}>
      <Input
        placeholder="Search by name or @username"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        autoComplete="off"
      />
      <div className="max-h-48 overflow-y-auto rounded-md border border-ink-200 bg-white">
        {filtered.length === 0 ? (
          <p className="px-3 py-2 text-sm text-ink-500">No matches.</p>
        ) : (
          <ul className="divide-y divide-ink-100">
            {filtered.map((f) => {
              const selected = picked === f.username;
              return (
                <li key={f.username}>
                  <button
                    type="button"
                    aria-pressed={selected}
                    onClick={() => setPicked(f.username)}
                    className={[
                      "flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm transition-colors",
                      selected
                        ? "bg-ink-100 text-ink-950"
                        : "text-ink-700 hover:bg-ink-50",
                    ].join(" ")}
                  >
                    <span className="min-w-0 truncate">
                      <span className="font-medium">
                        {f.displayName || f.username}
                      </span>
                      <span className="ml-1 text-ink-500">@{f.username}</span>
                    </span>
                    {selected && (
                      <span className="ml-2 shrink-0 text-[10px] uppercase tracking-wider text-ink-500">
                        Selected
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

function RolePills({
  className,
  picked,
  setPicked,
}: {
  className?: string;
  picked: Role;
  setPicked: (r: Role) => void;
}) {
  return (
    <div
      role="radiogroup"
      aria-label="Invite role"
      className={[className ?? "", "grid grid-cols-1 gap-2 min-[380px]:grid-cols-2 sm:flex sm:flex-wrap"].join(" ")}
    >
      {INVITEABLE_ROLES.map((r) => {
        const selected = picked === r;
        return (
          <button
            key={r}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => setPicked(r)}
            className={[
              "rounded-full border px-3 py-1 text-center text-xs font-medium uppercase tracking-wider transition-colors",
              selected
                ? "border-ink-950 bg-ink-950 text-white"
                : "border-ink-200 bg-white text-ink-600 hover:border-ink-400",
            ].join(" ")}
          >
            {prettyRole(r)}
          </button>
        );
      })}
    </div>
  );
}

function MembersCard({
  className,
  members,
  viewerIsOwner,
  busyMember,
  memberError,
  onCancelEdit,
  role,
  tags,
}: {
  className: string;
  members: Member[];
  viewerIsOwner: boolean;
  busyMember: string | null;
  memberError: string | null;
  onCancelEdit: () => void;
  role: RoleEditingProps;
  tags: TagsEditingProps;
}) {
  // Disable every editor control while any role or tag PATCH is in flight.
  const locked = busyMember !== null;

  return (
    <Card className={className}>
      <h2 className="text-lg font-medium">Members</h2>
      {memberError && (
        <p className="mt-2 text-sm text-red-600">{memberError}</p>
      )}
      <ul className="mt-4 divide-y divide-ink-100">
        {members.map((m) => {
          const canEditRole = viewerIsOwner && m.role !== "owner";
          const canEditTags = viewerIsOwner; // tags are editable for everyone, including the owner
          const isRoleExpanded =
            canEditRole && role.expandedMember === m.username;
          const isTagsExpanded =
            canEditTags && tags.editingMember === m.username;

          return (
            <li key={m.username} className="space-y-2 py-3">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{m.displayName}</p>
                  <p className="text-xs text-ink-500">@{m.username}</p>
                </div>
                {isRoleExpanded ? (
                  <div
                    role="radiogroup"
                    aria-label={`Change role for @${m.username}`}
                    className="grid grid-cols-1 gap-2 min-[380px]:grid-cols-2 sm:flex sm:flex-wrap sm:items-center"
                  >
                    {INVITEABLE_ROLES.map((r) => {
                      const selected = m.role === r;
                      return (
                        <button
                          key={r}
                          type="button"
                          role="radio"
                          aria-checked={selected}
                          onClick={() => role.onChange(m.username, r)}
                          disabled={locked || selected}
                          className={[
                            "rounded-full border px-3 py-1 text-center text-xs font-medium uppercase tracking-wider transition-colors",
                            selected
                              ? "border-ink-950 bg-ink-950 text-white"
                              : "border-ink-200 bg-white text-ink-600 hover:border-ink-400",
                            "disabled:cursor-default disabled:opacity-60",
                          ].join(" ")}
                        >
                          {prettyRole(r)}
                        </button>
                      );
                    })}
                    <button
                      type="button"
                      onClick={onCancelEdit}
                      disabled={locked}
                      className="px-2 py-1 text-xs text-ink-500 hover:text-ink-950 disabled:opacity-60"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded border border-ink-200 px-2 py-0.5 text-xs uppercase tracking-wider text-ink-500">
                      {prettyRole(m.role)}
                    </span>
                    {canEditRole && (
                      <button
                        type="button"
                        onClick={() => role.onOpen(m.username)}
                        disabled={locked}
                        className="text-xs text-ink-500 hover:text-ink-950 disabled:cursor-default disabled:opacity-50 disabled:hover:text-ink-500"
                      >
                        Change
                      </button>
                    )}
                  </div>
                )}
              </div>

              {isTagsExpanded ? (
                <TagEditor
                  username={m.username}
                  draft={tags.draft}
                  input={tags.input}
                  setInput={tags.setInput}
                  onKeyDown={tags.onKeyDown}
                  onRemove={tags.onRemove}
                  onSave={tags.onSave}
                  onCancel={onCancelEdit}
                  locked={locked}
                />
              ) : canEditTags ? (
                <div className="flex flex-wrap items-center gap-2">
                  {m.tags.length === 0 ? (
                    <span className="text-xs italic text-ink-400">
                      No tags
                    </span>
                  ) : (
                    m.tags.map((t) => <TagChip key={t} text={t} />)
                  )}
                  <button
                    type="button"
                    onClick={() => tags.onOpen(m.username, m.tags)}
                    disabled={locked}
                    className="text-xs text-ink-500 hover:text-ink-950 disabled:cursor-default disabled:opacity-50 disabled:hover:text-ink-500"
                  >
                    Edit tags
                  </button>
                </div>
              ) : m.tags.length > 0 ? (
                <div className="flex flex-wrap items-center gap-2">
                  {m.tags.map((t) => (
                    <TagChip key={t} text={t} />
                  ))}
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
    </Card>
  );
}

function TagChip({ text }: { text: string }) {
  return (
    <span className="inline-flex items-center rounded-full border border-ink-200 bg-white px-2 py-0.5 text-xs text-ink-600">
      {text}
    </span>
  );
}

function TagEditor({
  username,
  draft,
  input,
  setInput,
  onKeyDown,
  onRemove,
  onSave,
  onCancel,
  locked,
}: {
  username: string;
  draft: string[];
  input: string;
  setInput: (v: string) => void;
  onKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
  onRemove: (t: string) => void;
  onSave: (username: string) => void;
  onCancel: () => void;
  locked: boolean;
}) {
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {draft.length === 0 ? (
          <span className="text-xs italic text-ink-400">No tags yet.</span>
        ) : (
          draft.map((t) => (
            <span
              key={t}
              className="inline-flex items-center gap-1 rounded-full border border-ink-200 bg-white px-2 py-0.5 text-xs text-ink-700"
            >
              {t}
              <button
                type="button"
                onClick={() => onRemove(t)}
                disabled={locked}
                aria-label={`Remove tag ${t}`}
                className="ml-0.5 rounded-full text-ink-400 hover:text-ink-950 disabled:opacity-50"
              >
                ×
              </button>
            </span>
          ))
        )}
      </div>
      <Input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder="Add tag — press Enter"
        disabled={locked}
        maxLength={MAX_TAG_LEN}
        autoComplete="off"
      />
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Button
          type="button"
          onClick={() => onSave(username)}
          disabled={locked}
          className="w-full sm:w-auto"
        >
          {locked ? "Saving…" : "Save"}
        </Button>
        <button
          type="button"
          onClick={onCancel}
          disabled={locked}
          className="px-3 py-2 text-xs text-ink-500 hover:text-ink-950 disabled:opacity-50"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

function PlannerCard({ title, hint }: { title: string; hint: string }) {
  return (
    <Card className="p-4 opacity-70 sm:p-6">
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

// ---------------------------------------------------------------------------
// tag helpers — mirror backend normalization for UX, backend is still source
// of truth.
// ---------------------------------------------------------------------------

function normalizeTag(raw: string): string {
  // trim, lowercase, collapse internal whitespace into a single dash.
  const tokens = raw.trim().toLowerCase().split(/\s+/).filter(Boolean);
  return tokens.join("-");
}

function appendTag(
  tags: string[],
  raw: string,
): { tags: string[]; error?: string } {
  const norm = normalizeTag(raw);
  if (norm === "") return { tags };
  if (norm.length > MAX_TAG_LEN) {
    return { tags, error: `Tag is too long (max ${MAX_TAG_LEN}).` };
  }
  if (!TAG_PATTERN.test(norm)) {
    return {
      tags,
      error: "Tags can only use lowercase letters, digits, dashes, or underscores.",
    };
  }
  if (tags.includes(norm)) return { tags }; // silent dedupe
  if (tags.length >= MAX_TAGS) {
    return { tags, error: `A member can have at most ${MAX_TAGS} tags.` };
  }
  return { tags: [...tags, norm] };
}
