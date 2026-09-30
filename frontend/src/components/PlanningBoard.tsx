import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ChangeEvent, FormEvent, ReactNode } from "react";
import { DndContext, DragOverlay, KeyboardSensor, MouseSensor, TouchSensor, pointerWithin, rectIntersection, useDraggable, useDroppable, useSensor, useSensors } from "@dnd-kit/core";
import type { DragEndEvent, KeyboardCoordinateGetter } from "@dnd-kit/core";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "./ui/Button";
import { Card } from "./ui/Card";
import { Input } from "./ui/Input";
import { api, ApiError } from "../lib/api";
import { useConfirmation } from "./ui/ConfirmationDialog";

type Status = "todo" | "in_progress" | "done";
type Priority = "low" | "normal" | "high";

const STATUSES: Status[] = ["todo", "in_progress", "done"];
const PRIORITIES: Priority[] = ["low", "normal", "high"];

const columnKeyboardCoordinates: KeyboardCoordinateGetter = (event, { currentCoordinates, context }) => {
  const direction = ["ArrowRight", "ArrowDown"].includes(event.code) ? 1 : ["ArrowLeft", "ArrowUp"].includes(event.code) ? -1 : 0;
  if (!direction) return;
  event.preventDefault();
  const width = context.collisionRect?.width ?? 0;
  const height = context.collisionRect?.height ?? 0;
  const center = { x: currentCoordinates.x + width / 2, y: currentCoordinates.y + height / 2 };
  const index = STATUSES.findIndex((status) => {
    const rect = context.droppableRects.get(status);
    return rect && center.x >= rect.left && center.x <= rect.right && center.y >= rect.top && center.y <= rect.bottom;
  });
  const rect = context.droppableRects.get(STATUSES[index + direction]);
  return rect ? { x: rect.left + (rect.width - width) / 2, y: rect.top + (rect.height - height) / 2 } : undefined;
};

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
  status: Status;
  title: string;
  description: string;
  priority: Priority;
  assigneeUsername: string;
  dueDate: string; // YYYY-MM-DD (native date input)
};

const emptyForm: FormState = {
  status: "todo",
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
  canManage,
  reloadTrip,
}: {
  tripSlug: string | undefined;
  members: BoardMember[];
  viewerIsMember: boolean;
  canManage: boolean;
  // reloadTrip refreshes the parent dashboard's trip + members. We call
  // it when a task action returns 403/404 so viewerIsMember can re-derive
  // from the server's view of membership.
  reloadTrip: () => Promise<void>;
}) {
  const confirm = useConfirmation();
  const [tasks, setTasks] = useState<Task[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // accessLost flips on when a task action returns 403/404, hiding every
  // task control until either the parent reload says we're no longer a
  // member (the existing non-member empty state takes over) or a follow-up
  // GET succeeds (transient — recover and clear).
  const [accessLost, setAccessLost] = useState(false);

  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [formError, setFormError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  // busyTaskId locks status/delete buttons across every row while any task
  // PATCH/DELETE is in flight, so a second click cannot race the first.
  const [busyTaskId, setBusyTaskId] = useState<string | null>(null);
  const [activeTask, setActiveTask] = useState<Task | null>(null);
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 180, tolerance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: columnKeyboardCoordinates }),
  );

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
    setAccessLost(false);
    setShowForm(false);
    setForm(emptyForm);
    setFormError(null);
    setCreating(false);
    setBusyTaskId(null);
    setActiveTask(null);
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
      // Re-sort every bucket on the client so a PATCH that rebuckets a
      // task (backend assigns a new position at the end of the new
      // column) always renders in the correct order, even though we
      // splice the response in place instead of refetching.
      for (const s of STATUSES) {
        out[s].sort(compareTasks);
      }
    }
    return out;
  }, [tasks]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (creating || accessLost || !canManage) return;

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
        status: form.status,
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
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        setShowForm(false);
        await handleAccessError(gen, err.message);
      } else {
        setFormError(err instanceof ApiError ? err.message : "Create failed");
      }
    } finally {
      if (gen === genRef.current) setCreating(false);
    }
  }

  // handleAccessError centralizes the 403/404 path used by both PATCH
  // and DELETE. It hides task controls locally, asks the parent to
  // refetch trip + members (so viewerIsMember can re-derive), then
  // attempts a single follow-up GET. If the GET succeeds, the error was
  // transient and we recover; if it fails again, accessLost stays true
  // and controls remain hidden until the user navigates.
  const handleAccessError = useCallback(
    async (gen: number, message: string) => {
      if (gen !== genRef.current) return;
      setActionError(message);
      setAccessLost(true);
      await reloadTrip();
      if (gen !== genRef.current || !tripSlug) return;
      try {
        const xs = await api<Task[]>("GET", `/trips/${tripSlug}/tasks`);
        if (gen !== genRef.current) return;
        setTasks(xs);
        setAccessLost(false);
        setActionError(null);
        setLoadError(null);
      } catch {
        // Still no access — leave accessLost true. If viewerIsMember
        // flipped to false, the reset effect already cleared state.
      }
    },
    [reloadTrip, tripSlug],
  );

  async function onChangeStatus(task: Task, next: Status) {
    if (busyTaskId !== null || accessLost || next === task.status || !tripSlug || !canManage) return;
    const gen = genRef.current;
    setBusyTaskId(task.id);
    setActionError(null);
    const position = Math.max(-1, ...(tasks ?? []).filter((t) => t.status === next).map((t) => t.position)) + 1;
    setTasks((prev) => prev?.map((t) => t.id === task.id ? { ...t, status: next, position } : t) ?? prev);
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
      setTasks((prev) => prev?.map((t) => t.id === task.id ? task : t) ?? prev);
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await handleAccessError(gen, err.message);
      } else {
        setActionError(
          err instanceof ApiError ? err.message : "Update failed",
        );
      }
    } finally {
      if (gen === genRef.current) setBusyTaskId(null);
    }
  }

  function openCreate(status: Status) {
    setForm((prev) => showForm ? { ...prev, status } : { ...emptyForm, status });
    setFormError(null);
    setShowForm(true);
  }

  function onDragEnd({ active, over }: DragEndEvent) {
    setActiveTask(null);
    const task = tasks?.find((t) => t.id === active.id);
    if (task && over && STATUSES.includes(over.id as Status)) {
      void onChangeStatus(task, over.id as Status);
    }
  }

  async function onDelete(task: Task) {
    if (busyTaskId !== null || accessLost || !tripSlug || !canManage) return;
    const gen = genRef.current;
    if (!await confirm({ title: "Delete task?", description: `"${task.title}" will be permanently deleted.` }) || gen !== genRef.current) return;
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
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await handleAccessError(gen, err.message);
      } else {
        setActionError(
          err instanceof ApiError ? err.message : "Delete failed",
        );
      }
    } finally {
      if (gen === genRef.current) setBusyTaskId(null);
    }
  }

  return (
    <section className="mt-10">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-baseline sm:justify-between">
        <div>
          <h2 className="text-lg font-medium">Planning</h2>
          <p className="text-xs text-ink-500">
            Lightweight tasks for everything that needs to happen before the
            trip.
          </p>
        </div>
        {viewerIsMember && canManage && !accessLost && (
          <Button
            variant={showForm ? "ghost" : "primary"}
            onClick={() => {
              if (showForm) setShowForm(false);
              else openCreate("todo");
            }}
            className="w-full sm:w-auto"
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
      ) : accessLost ? (
        <Card className="mt-4 p-6 text-center">
          <p className="text-sm text-ink-500">
            You no longer have access to manage planning tasks for this
            trip.
          </p>
          {actionError && (
            <p className="mt-2 text-xs text-ink-400">{actionError}</p>
          )}
        </Card>
      ) : (
        <>
          {!canManage && (
            <p className="mt-3 text-xs text-ink-500">
              You can view planning tasks. Planner, admin, or owner role is
              required to change them.
            </p>
          )}

          {actionError && (
            <p className="mt-3 text-sm text-red-600">{actionError}</p>
          )}

          {showForm && (
            <Card className="mt-4 p-4 sm:p-5">
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
            <DndContext
              sensors={sensors}
              collisionDetection={(args) => {
                const collisions = pointerWithin(args);
                return args.pointerCoordinates ? collisions : rectIntersection(args);
              }}
              onDragStart={({ active }) => setActiveTask(tasks.find((t) => t.id === active.id) ?? null)}
              onDragEnd={onDragEnd}
              onDragCancel={() => setActiveTask(null)}
            >
            <div className="mt-4 grid gap-4 lg:grid-cols-3">
              {STATUSES.map((status) => (
                <TaskColumn key={status} status={status} disabled={!canManage || busyTaskId !== null}>
                  <div className="flex items-baseline justify-between gap-3">
                    <h3 className="text-sm font-medium">
                      {prettyStatus(status)}
                    </h3>
                    <div className="flex items-center gap-2">
                    <span className="text-[10px] text-ink-400">
                      {byStatus[status].length}
                    </span>
                    {canManage && <button type="button" onClick={() => openCreate(status)} aria-label={`Add task to ${prettyStatus(status)}`} title={`Add task to ${prettyStatus(status)}`} className="flex h-7 w-7 items-center justify-center rounded hover:bg-ink-100"><Plus size={16} /></button>}
                    </div>
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
                          canManage={canManage}
                          onChangeStatus={onChangeStatus}
                          onDelete={onDelete}
                        />
                      ))
                    )}
                  </div>
                </TaskColumn>
              ))}
            </div>
            <DragOverlay dropAnimation={{ duration: 180, easing: "ease-out" }}>
              {activeTask && <article aria-hidden="true" className="pointer-events-none rounded-md border border-ink-200 bg-white p-3 shadow-lg"><TaskContent task={activeTask} locked canManage={canManage} onChangeStatus={() => {}} onDelete={() => {}} /></article>}
            </DragOverlay>
            </DndContext>
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
        <label htmlFor="task-status" className="text-xs font-medium text-ink-600">Status</label>
        <select id="task-status" value={form.status} disabled={busy} onChange={(e) => setField("status", e.target.value as Status)} className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm">
          {STATUSES.map((status) => <option key={status} value={status}>{prettyStatus(status)}</option>)}
        </select>
      </div>
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
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
      <Button type="submit" disabled={busy} className="w-full sm:w-auto">
        {busy ? "Adding…" : "Add task"}
      </Button>
    </form>
  );
}

type TaskCardProps = {
  task: Task;
  locked: boolean;
  canManage: boolean;
  onChangeStatus: (task: Task, next: Status) => void;
  onDelete: (task: Task) => void;
};

function TaskColumn({ status, disabled, children }: { status: Status; disabled: boolean; children: ReactNode }) {
  const { setNodeRef } = useDroppable({ id: status, disabled });
  return <section ref={setNodeRef} aria-label={prettyStatus(status)} data-task-column={status} className="min-h-36 bg-ink-50/70 p-3 sm:p-4">{children}</section>;
}

function TaskCard(props: TaskCardProps) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: props.task.id, disabled: !props.canManage || props.locked });
  return (
    <article ref={setNodeRef} {...(props.canManage && !props.locked ? { ...attributes, ...listeners } : {})} aria-label={props.task.title} data-task-id={props.task.id} className={`rounded-md border border-ink-200 bg-white p-3 transition-opacity ${isDragging ? "opacity-30" : "opacity-100"} ${props.canManage && !props.locked ? "cursor-grab active:cursor-grabbing" : ""}`}>
      <TaskContent {...props} />
    </article>
  );
}

function TaskContent({
  task,
  locked,
  canManage,
  onChangeStatus,
  onDelete,
}: TaskCardProps) {
  return (
    <>
      <div className="flex items-start justify-between gap-2">
        <p className="min-w-0 break-words text-sm font-medium leading-snug">
          {task.title}
        </p>
        <PriorityBadge p={task.priority} />
      </div>
      {task.description && (
        <p className="mt-1 break-words text-xs text-ink-500">
          {task.description}
        </p>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-ink-500">
        {task.assignee && (
          <span>
            @<span className="font-medium">{task.assignee.username}</span>
          </span>
        )}
        {task.dueDate && <span>Due {formatDueDate(task.dueDate)}</span>}
        <span className="text-ink-400">
          by @{task.createdBy.username}
        </span>
      </div>
      {canManage && (
      <div onMouseDown={(e) => e.stopPropagation()} onTouchStart={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()} className="mt-3 flex flex-wrap items-center gap-1 border-t border-ink-100 pt-3">
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
          aria-label={`Delete ${task.title}`}
          title="Delete task"
          className="ml-auto flex h-7 w-7 items-center justify-center rounded text-ink-400 hover:text-red-600 disabled:opacity-50"
        >
          <Trash2 size={14} />
        </button>
      </div>
      )}
    </>
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
    "max-w-full break-all text-center",
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

// compareTasks reproduces the backend's deterministic order so a single
// bucket renders the same way the server would return it: position asc,
// createdAt asc, id asc. RFC3339 strings compare correctly lexically, so
// no Date math is needed.
function compareTasks(a: Task, b: Task): number {
  if (a.position !== b.position) return a.position - b.position;
  if (a.createdAt !== b.createdAt) return a.createdAt < b.createdAt ? -1 : 1;
  if (a.id !== b.id) return a.id < b.id ? -1 : 1;
  return 0;
}

// formatDueDate formats a Postgres date column (returned by the API as
// an RFC3339 timestamp at midnight UTC) without going through Date,
// which would otherwise shift the displayed day for users in negative
// timezones. We keep only the YYYY-MM-DD prefix.
function formatDueDate(raw: string): string {
  const m = raw.match(/^(\d{4}-\d{2}-\d{2})/);
  return m ? m[1] : raw;
}
