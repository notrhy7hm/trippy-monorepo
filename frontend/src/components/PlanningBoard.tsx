import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ChangeEvent, FormEvent } from "react";
import { Button } from "./ui/Button";
import { Card } from "./ui/Card";
import { Input } from "./ui/Input";
import { api, ApiError } from "../lib/api";

type Status = "todo" | "in_progress" | "done";
type Priority = "low" | "normal" | "high";

const STATUSES: Status[] = ["todo", "in_progress", "done"];
const PRIORITIES: Priority[] = ["low", "normal", "high"];

type Party = { username: string; displayName: string };

type Task = {
  id: string;
  title: string;
  description: string;
  status: Status;
  priority: Priority;
  assignee?: Party;
  createdBy: Party;
  position: number;
  dueDate?: string;
  createdAt: string;
  updatedAt: string;
};

// Only what the board needs from a trip member, so callers can pass the
// existing Member type without remapping.
export type BoardMember = {
  username: string;
  displayName: string;
};

type FormState = {
  title: string;
  description: string;
  priority: Priority;
  assigneeUsername: string;
  dueDate: string; // YYYY-MM-DD (native date input)
};

const emptyForm: FormState = {
  title: "",
  description: "",
  priority: "normal",
  assigneeUsername: "",
  dueDate: "",
};

export function PlanningBoard({
  tripSlug,
  members,
  viewerIsMember,
}: {
  tripSlug: string | undefined;
  members: BoardMember[];
  viewerIsMember: boolean;
}) {
  const [tasks, setTasks] = useState<Task[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [formError, setFormError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  // busyTaskId locks status/delete buttons across every row while any task
  // PATCH/DELETE is in flight, so a second click cannot race the first.
  const [busyTaskId, setBusyTaskId] = useState<string | null>(null);

  // genRef ticks on every tripSlug change. Async resolves capture
  // genRef.current before awaiting and bail out if the slug has changed —
  // same pattern as TripDashboard / Friends.
  const genRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!tripSlug || !viewerIsMember) return;
    const gen = genRef.current;
    try {
      const xs = await api<Task[]>("GET", `/trips/${tripSlug}/tasks`);
      if (gen !== genRef.current) return;
      setTasks(xs);
      setLoadError(null);
    } catch (err) {
      if (gen !== genRef.current) return;
      // Keep any prior tasks visible if a refresh-after-mutate fails.
      setTasks((prev) => prev ?? []);
      setLoadError(
        err instanceof ApiError ? err.message : "Failed to load tasks",
      );
    }
  }, [tripSlug, viewerIsMember]);

  // Reset every board-local piece of state on tripSlug change, then load
  // the new trip's tasks. viewerIsMember is also a trigger because the
  // /tasks endpoint refuses non-members; once the caller becomes a member
  // we want to fetch.
  useEffect(() => {
    genRef.current++;
    setTasks(null);
    setLoadError(null);
    setActionError(null);
    setShowForm(false);
    setForm(emptyForm);
    setFormError(null);
    setCreating(false);
    setBusyTaskId(null);
    if (viewerIsMember && tripSlug) refresh();
  }, [tripSlug, viewerIsMember, refresh]);

  const byStatus = useMemo(() => {
    const out: Record<Status, Task[]> = {
      todo: [],
      in_progress: [],
      done: [],
    };
    if (tasks) {
      for (const t of tasks) {
        out[t.status].push(t);
      }
    }
    return out;
  }, [tasks]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (creating) return;

    const title = form.title.trim();
    if (title === "") {
      setFormError("Title is required.");
      return;
    }

    if (!tripSlug) return;
    const gen = genRef.current;
    setCreating(true);
    setFormError(null);
    try {
      const body: Record<string, unknown> = {
        title,
        description: form.description.trim(),
        priority: form.priority,
      };
      const assignee = form.assigneeUsername.trim();
      if (assignee !== "") body.assigneeUsername = assignee;
      if (form.dueDate !== "") body.dueDate = form.dueDate;

      const t = await api<Task>("POST", `/trips/${tripSlug}/tasks`, body);
      if (gen !== genRef.current) return;
      setTasks((prev) => (prev ? [...prev, t] : [t]));
      setForm(emptyForm);
      setShowForm(false);
    } catch (err) {
      if (gen !== genRef.current) return;
      setFormError(err instanceof ApiError ? err.message : "Create failed");
    } finally {
      if (gen === genRef.current) setCreating(false);
    }
  }

  async function onChangeStatus(task: Task, next: Status) {
    if (busyTaskId !== null || next === task.status || !tripSlug) return;
    const gen = genRef.current;
    setBusyTaskId(task.id);
    setActionError(null);
    try {
      const updated = await api<Task>(
        "PATCH",
        `/trips/${tripSlug}/tasks/${encodeURIComponent(task.id)}`,
        { status: next },
      );
      if (gen !== genRef.current) return;
      setTasks((prev) =>
        prev ? prev.map((x) => (x.id === task.id ? updated : x)) : prev,
      );
    } catch (err) {
      if (gen !== genRef.current) return;
      setActionError(
        err instanceof ApiError ? err.message : "Update failed",
      );
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await refresh();
      }
    } finally {
      if (gen === genRef.current) setBusyTaskId(null);
    }
  }

  async function onDelete(task: Task) {
    if (busyTaskId !== null || !tripSlug) return;
    if (!window.confirm(`Delete "${task.title}"?`)) return;
    const gen = genRef.current;
    setBusyTaskId(task.id);
    setActionError(null);
    try {
      await api(
        "DELETE",
        `/trips/${tripSlug}/tasks/${encodeURIComponent(task.id)}`,
      );
      if (gen !== genRef.current) return;
      setTasks((prev) => (prev ? prev.filter((x) => x.id !== task.id) : prev));
    } catch (err) {
      if (gen !== genRef.current) return;
      setActionError(
        err instanceof ApiError ? err.message : "Delete failed",
      );
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await refresh();
      }
    } finally {
      if (gen === genRef.current) setBusyTaskId(null);
    }
  }

  return (
    <section className="mt-10">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium">Planning</h2>
          <p className="text-xs text-ink-500">
            Lightweight tasks for everything that needs to happen before the
            trip.
          </p>
        </div>
        {viewerIsMember && (
          <Button
            variant={showForm ? "ghost" : "primary"}
            onClick={() => {
              setShowForm((v) => !v);
              setFormError(null);
            }}
          >
            {showForm ? "Cancel" : "Add task"}
          </Button>
        )}
      </div>

      {!viewerIsMember ? (
        <Card className="mt-4 p-6 text-center">
          <p className="text-sm text-ink-500">
            Only trip members can see and manage planning tasks.
          </p>
        </Card>
      ) : (
        <>
          {actionError && (
            <p className="mt-3 text-sm text-red-600">{actionError}</p>
          )}

          {showForm && (
            <Card className="mt-4 p-5">
              <CreateForm
                form={form}
                setForm={setForm}
                members={members}
                error={formError}
                busy={creating}
                onSubmit={onCreate}
              />
            </Card>
          )}

          {tasks === null && !loadError && (
            <p className="mt-4 text-sm text-ink-500">Loading…</p>
          )}

          {loadError && (
            <p className="mt-4 text-xs text-ink-500">
              Planning tasks are temporarily unavailable.
            </p>
          )}

          {tasks && (
            <div className="mt-4 grid gap-4 md:grid-cols-3">
              {STATUSES.map((status) => (
                <Card key={status} className="p-4">
                  <div className="flex items-baseline justify-between">
                    <h3 className="text-sm font-medium">
                      {prettyStatus(status)}
                    </h3>
                    <span className="text-[10px] uppercase tracking-wider text-ink-400">
                      {byStatus[status].length}
                    </span>
                  </div>
                  <div className="mt-3 space-y-3">
                    {byStatus[status].length === 0 ? (
                      <p className="text-xs italic text-ink-400">
                        No tasks yet.
                      </p>
                    ) : (
                      byStatus[status].map((t) => (
                        <TaskCard
                          key={t.id}
                          task={t}
                          locked={busyTaskId !== null}
                          onChangeStatus={onChangeStatus}
                          onDelete={onDelete}
                        />
                      ))
                    )}
                  </div>
                </Card>
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}

function CreateForm({
  form,
  setForm,
  members,
  error,
  busy,
  onSubmit,
}: {
  form: FormState;
  setForm: (next: FormState) => void;
  members: BoardMember[];
  error: string | null;
  busy: boolean;
  onSubmit: (e: FormEvent) => void;
}) {
  function setField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm({ ...form, [key]: value });
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div>
        <label className="text-xs font-medium text-ink-600">Title</label>
        <Input
          className="mt-1"
          value={form.title}
          onChange={(e) => setField("title", e.target.value)}
          placeholder="Book hotel"
          autoFocus
          required
        />
      </div>
      <div>
        <label className="text-xs font-medium text-ink-600">
          Description
        </label>
        <Input
          className="mt-1"
          value={form.description}
          onChange={(e) => setField("description", e.target.value)}
          placeholder="Optional"
        />
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <span className="text-xs font-medium text-ink-600">Priority</span>
          <div
            role="radiogroup"
            aria-label="Priority"
            className="mt-1 flex flex-wrap gap-2"
          >
            {PRIORITIES.map((p) => {
              const selected = form.priority === p;
              return (
                <button
                  key={p}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  onClick={() => setField("priority", p)}
                  className={pillClass(selected)}
                >
                  {prettyPriority(p)}
                </button>
              );
            })}
          </div>
        </div>
        <div>
          <label className="text-xs font-medium text-ink-600">Due date</label>
          <input
            type="date"
            value={form.dueDate}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              setField("dueDate", e.target.value)
            }
            className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
          />
        </div>
      </div>
      <div>
        <span className="text-xs font-medium text-ink-600">Assignee</span>
        <div className="mt-1 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => setField("assigneeUsername", "")}
            className={pillClass(form.assigneeUsername === "")}
          >
            Unassigned
          </button>
          {members.map((m) => {
            const selected = form.assigneeUsername === m.username;
            return (
              <button
                key={m.username}
                type="button"
                onClick={() => setField("assigneeUsername", m.username)}
                className={pillClass(selected)}
                title={m.displayName}
              >
                @{m.username}
              </button>
            );
          })}
        </div>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      <Button type="submit" disabled={busy}>
        {busy ? "Adding…" : "Add task"}
      </Button>
    </form>
  );
}

function TaskCard({
  task,
  locked,
  onChangeStatus,
  onDelete,
}: {
  task: Task;
  locked: boolean;
  onChangeStatus: (task: Task, next: Status) => void;
  onDelete: (task: Task) => void;
}) {
  return (
    <article className="rounded-md border border-ink-200 bg-white p-3">
      <div className="flex items-start justify-between gap-2">
        <p className="text-sm font-medium leading-snug">{task.title}</p>
        <PriorityBadge p={task.priority} />
      </div>
      {task.description && (
        <p className="mt-1 text-xs text-ink-500">{task.description}</p>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-ink-500">
        {task.assignee && (
          <span>
            @<span className="font-medium">{task.assignee.username}</span>
          </span>
        )}
        {task.dueDate && (
          <span>Due {new Date(task.dueDate).toLocaleDateString()}</span>
        )}
        <span className="text-ink-400">
          by @{task.createdBy.username}
        </span>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1 border-t border-ink-100 pt-3">
        {STATUSES.map((s) => {
          const selected = task.status === s;
          return (
            <button
              key={s}
              type="button"
              onClick={() => onChangeStatus(task, s)}
              disabled={locked || selected}
              aria-pressed={selected}
              className={[
                "rounded-full border px-2 py-0.5 text-[10px] uppercase tracking-wider transition-colors",
                selected
                  ? "border-ink-950 bg-ink-950 text-white"
                  : "border-ink-200 bg-white text-ink-600 hover:border-ink-400",
                "disabled:cursor-default disabled:opacity-60",
              ].join(" ")}
            >
              {prettyStatus(s)}
            </button>
          );
        })}
        <button
          type="button"
          onClick={() => onDelete(task)}
          disabled={locked}
          className="ml-auto text-[10px] uppercase tracking-wider text-ink-400 hover:text-red-600 disabled:opacity-50 disabled:hover:text-ink-400"
        >
          Delete
        </button>
      </div>
    </article>
  );
}

function PriorityBadge({ p }: { p: Priority }) {
  return (
    <span className="shrink-0 rounded border border-ink-200 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-ink-500">
      {prettyPriority(p)}
    </span>
  );
}

function pillClass(selected: boolean) {
  return [
    "rounded-full border px-3 py-1 text-xs font-medium uppercase tracking-wider transition-colors",
    selected
      ? "border-ink-950 bg-ink-950 text-white"
      : "border-ink-200 bg-white text-ink-600 hover:border-ink-400",
  ].join(" ");
}

function prettyStatus(s: Status) {
  if (s === "in_progress") return "In progress";
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function prettyPriority(p: Priority) {
  return p.charAt(0).toUpperCase() + p.slice(1);
}
