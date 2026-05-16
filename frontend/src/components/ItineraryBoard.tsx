import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { ChangeEvent, FormEvent, ReactNode } from "react";
import { Button } from "./ui/Button";
import { Card } from "./ui/Card";
import { Input } from "./ui/Input";
import { api, ApiError } from "../lib/api";

type Party = { username: string; displayName: string };

type ItineraryItem = {
  id: string;
  dayIndex: number;
  date?: string; // YYYY-MM-DD or RFC3339-prefix
  title: string;
  notes: string;
  locationName?: string;
  startsAt?: string; // HH:MM:SS
  endsAt?: string; // HH:MM:SS
  position: number;
  createdBy: Party;
  createdAt: string;
  updatedAt: string;
};

export type ItineraryMember = {
  username: string;
  displayName: string;
};

// FormState is shared by create and edit — same shape, just a different
// instance for each mode so neither can stomp the other.
type FormState = {
  dayIndex: string;
  date: string;
  title: string;
  notes: string;
  locationName: string;
  startsAt: string; // HH:MM
  endsAt: string;
};

const emptyForm: FormState = {
  dayIndex: "0",
  date: "",
  title: "",
  notes: "",
  locationName: "",
  startsAt: "",
  endsAt: "",
};

export function ItineraryBoard({
  tripSlug,
  viewerIsMember,
  reloadTrip,
}: {
  tripSlug: string | undefined;
  // Members is currently unused by this section (no assignee), but kept
  // in the prop list so the dashboard can extend it without changing
  // signatures later.
  members: ItineraryMember[];
  viewerIsMember: boolean;
  reloadTrip: () => Promise<void>;
}) {
  const [items, setItems] = useState<ItineraryItem[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [accessLost, setAccessLost] = useState(false);

  const [showForm, setShowForm] = useState(false);
  const [createForm, setCreateForm] = useState<FormState>(emptyForm);
  const [createError, setCreateError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<FormState>(emptyForm);
  const [editError, setEditError] = useState<string | null>(null);

  // busyItemId locks every item action across the whole section while one
  // PATCH/DELETE is in flight, matching PlanningBoard's pattern.
  const [busyItemId, setBusyItemId] = useState<string | null>(null);

  // genRef ticks on every tripSlug change. Async resolves capture
  // genRef.current before awaiting and bail if the slug has changed.
  const genRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!tripSlug || !viewerIsMember) return;
    const gen = genRef.current;
    try {
      const xs = await api<ItineraryItem[]>(
        "GET",
        `/trips/${tripSlug}/itinerary`,
      );
      if (gen !== genRef.current) return;
      setItems(xs);
      setLoadError(null);
    } catch (err) {
      if (gen !== genRef.current) return;
      setItems((prev) => prev ?? []);
      setLoadError(
        err instanceof ApiError ? err.message : "Failed to load itinerary",
      );
    }
  }, [tripSlug, viewerIsMember]);

  useEffect(() => {
    genRef.current++;
    setItems(null);
    setLoadError(null);
    setActionError(null);
    setAccessLost(false);
    setShowForm(false);
    setCreateForm(emptyForm);
    setCreateError(null);
    setCreating(false);
    setEditingItemId(null);
    setEditForm(emptyForm);
    setEditError(null);
    setBusyItemId(null);
    if (viewerIsMember && tripSlug) refresh();
  }, [tripSlug, viewerIsMember, refresh]);

  // Group + sort. Same comparator the backend uses (position asc, then
  // startsAt asc nulls last, then createdAt asc, then id asc) so render
  // order matches list order even after mutations splice in place.
  const groups = useMemo(() => buildGroups(items ?? []), [items]);

  // ---- centralized 403/404 handling for itinerary actions ---------------
  const handleAccessError = useCallback(
    async (gen: number, message: string) => {
      if (gen !== genRef.current) return;
      setActionError(message);
      setAccessLost(true);
      // Close any open editor; it would otherwise be visually stale.
      setShowForm(false);
      setEditingItemId(null);
      await reloadTrip();
      if (gen !== genRef.current || !tripSlug) return;
      try {
        const xs = await api<ItineraryItem[]>(
          "GET",
          `/trips/${tripSlug}/itinerary`,
        );
        if (gen !== genRef.current) return;
        setItems(xs);
        setAccessLost(false);
        setActionError(null);
        setLoadError(null);
      } catch {
        // Still no access. accessLost stays on; if viewerIsMember flips
        // false, the reset effect already cleared state.
      }
    },
    [reloadTrip, tripSlug],
  );

  // ---- create -----------------------------------------------------------
  function openCreate() {
    setShowForm(true);
    setEditingItemId(null);
    setCreateForm(emptyForm);
    setCreateError(null);
  }
  function closeCreate() {
    setShowForm(false);
    setCreateError(null);
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    if (creating || accessLost || !tripSlug) return;

    const validated = validateForm(createForm);
    if (!validated.ok) {
      setCreateError(validated.error);
      return;
    }

    const gen = genRef.current;
    setCreating(true);
    setCreateError(null);
    try {
      const body = buildCreateBody(validated.values, createForm);
      const created = await api<ItineraryItem>(
        "POST",
        `/trips/${tripSlug}/itinerary`,
        body,
      );
      if (gen !== genRef.current) return;
      setItems((prev) => (prev ? [...prev, created] : [created]));
      setCreateForm(emptyForm);
      setShowForm(false);
    } catch (err) {
      if (gen !== genRef.current) return;
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await handleAccessError(gen, err.message);
      } else {
        setCreateError(
          err instanceof ApiError ? err.message : "Create failed",
        );
      }
    } finally {
      if (gen === genRef.current) setCreating(false);
    }
  }

  // ---- edit -------------------------------------------------------------
  function openEdit(item: ItineraryItem) {
    setShowForm(false);
    setEditingItemId(item.id);
    setEditForm({
      dayIndex: String(item.dayIndex),
      date: item.date ? formatDateInput(item.date) : "",
      title: item.title,
      notes: item.notes,
      locationName: item.locationName ?? "",
      startsAt: item.startsAt ? formatTimeInput(item.startsAt) : "",
      endsAt: item.endsAt ? formatTimeInput(item.endsAt) : "",
    });
    setEditError(null);
  }
  function closeEdit() {
    setEditingItemId(null);
    setEditForm(emptyForm);
    setEditError(null);
  }

  async function onSaveEdit(e: FormEvent) {
    e.preventDefault();
    if (busyItemId !== null || accessLost || !tripSlug || !editingItemId) {
      return;
    }
    const validated = validateForm(editForm);
    if (!validated.ok) {
      setEditError(validated.error);
      return;
    }

    const gen = genRef.current;
    setBusyItemId(editingItemId);
    setEditError(null);
    try {
      // Send full set of editable fields; "" clears nullable columns,
      // matching the backend's PATCH semantics.
      const body = {
        dayIndex: validated.values.dayIndex,
        title: validated.values.title,
        notes: editForm.notes,
        locationName: editForm.locationName.trim(),
        date: editForm.date,
        startsAt: editForm.startsAt,
        endsAt: editForm.endsAt,
      };
      const updated = await api<ItineraryItem>(
        "PATCH",
        `/trips/${tripSlug}/itinerary/${encodeURIComponent(editingItemId)}`,
        body,
      );
      if (gen !== genRef.current) return;
      setItems((prev) =>
        prev ? prev.map((x) => (x.id === updated.id ? updated : x)) : prev,
      );
      closeEdit();
    } catch (err) {
      if (gen !== genRef.current) return;
      if (
        err instanceof ApiError &&
        (err.status === 403 || err.status === 404)
      ) {
        await handleAccessError(gen, err.message);
      } else {
        setEditError(err instanceof ApiError ? err.message : "Save failed");
      }
    } finally {
      if (gen === genRef.current) setBusyItemId(null);
    }
  }

  // ---- delete -----------------------------------------------------------
  async function onDelete(item: ItineraryItem) {
    if (busyItemId !== null || accessLost || !tripSlug) return;
    if (!window.confirm(`Delete "${item.title}"?`)) return;
    const gen = genRef.current;
    setBusyItemId(item.id);
    setActionError(null);
    try {
      await api(
        "DELETE",
        `/trips/${tripSlug}/itinerary/${encodeURIComponent(item.id)}`,
      );
      if (gen !== genRef.current) return;
      setItems((prev) =>
        prev ? prev.filter((x) => x.id !== item.id) : prev,
      );
      if (editingItemId === item.id) closeEdit();
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
      if (gen === genRef.current) setBusyItemId(null);
    }
  }

  const locked = busyItemId !== null;

  return (
    <section className="mt-10">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium">Itinerary</h2>
          <p className="text-xs text-ink-500">
            Day-by-day plan for this trip.
          </p>
        </div>
        {viewerIsMember && !accessLost && (
          <Button
            variant={showForm ? "ghost" : "primary"}
            onClick={() => (showForm ? closeCreate() : openCreate())}
          >
            {showForm ? "Cancel" : "Add item"}
          </Button>
        )}
      </div>

      {!viewerIsMember ? (
        <Card className="mt-4 p-6 text-center">
          <p className="text-sm text-ink-500">
            Only trip members can see and manage the itinerary.
          </p>
        </Card>
      ) : accessLost ? (
        <Card className="mt-4 p-6 text-center">
          <p className="text-sm text-ink-500">
            You no longer have access to manage the itinerary for this
            trip.
          </p>
          {actionError && (
            <p className="mt-2 text-xs text-ink-400">{actionError}</p>
          )}
        </Card>
      ) : (
        <>
          {actionError && (
            <p className="mt-3 text-sm text-red-600">{actionError}</p>
          )}

          {showForm && (
            <Card className="mt-4 p-5">
              <ItineraryFormFields
                form={createForm}
                setForm={setCreateForm}
                error={createError}
                busy={creating}
                submitLabel={creating ? "Adding…" : "Add item"}
                onSubmit={onCreate}
                onCancel={closeCreate}
              />
            </Card>
          )}

          {items === null && !loadError && (
            <p className="mt-4 text-sm text-ink-500">Loading…</p>
          )}

          {loadError && items !== null && (
            <p className="mt-4 text-xs text-ink-500">
              Itinerary is temporarily unavailable.
            </p>
          )}

          {items && items.length === 0 && !showForm && (
            <Card className="mt-4 p-6 text-center">
              <p className="text-sm text-ink-500">
                No itinerary items yet. Use Add item to start your plan.
              </p>
            </Card>
          )}

          {items && items.length > 0 && (
            <div className="mt-6 space-y-6">
              {groups.map((g) => (
                <DayGroup
                  key={g.dayIndex}
                  group={g}
                  editingItemId={editingItemId}
                  editForm={editForm}
                  setEditForm={setEditForm}
                  editError={editError}
                  locked={locked}
                  onEdit={openEdit}
                  onCancelEdit={closeEdit}
                  onSaveEdit={onSaveEdit}
                  onDelete={onDelete}
                />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------
// day groups
// ---------------------------------------------------------------------------

type DayBucket = {
  dayIndex: number;
  date?: string; // representative date (first non-null in the group)
  items: ItineraryItem[];
};

function DayGroup({
  group,
  editingItemId,
  editForm,
  setEditForm,
  editError,
  locked,
  onEdit,
  onCancelEdit,
  onSaveEdit,
  onDelete,
}: {
  group: DayBucket;
  editingItemId: string | null;
  editForm: FormState;
  setEditForm: (next: FormState) => void;
  editError: string | null;
  locked: boolean;
  onEdit: (item: ItineraryItem) => void;
  onCancelEdit: () => void;
  onSaveEdit: (e: FormEvent) => void;
  onDelete: (item: ItineraryItem) => void;
}) {
  return (
    <div>
      <h3 className="text-sm font-medium text-ink-700">
        Day {group.dayIndex}
        {group.date && (
          <span className="ml-2 text-xs font-normal text-ink-400">
            · {group.date}
          </span>
        )}
      </h3>
      <div className="mt-2 space-y-2">
        {group.items.map((item) =>
          editingItemId === item.id ? (
            <Card key={item.id} className="p-4">
              <ItineraryFormFields
                form={editForm}
                setForm={setEditForm}
                error={editError}
                busy={locked}
                submitLabel={locked ? "Saving…" : "Save"}
                onSubmit={onSaveEdit}
                onCancel={onCancelEdit}
              />
            </Card>
          ) : (
            <ItineraryItemCard
              key={item.id}
              item={item}
              locked={locked}
              onEdit={() => onEdit(item)}
              onDelete={() => onDelete(item)}
            />
          ),
        )}
      </div>
    </div>
  );
}

function ItineraryItemCard({
  item,
  locked,
  onEdit,
  onDelete,
}: {
  item: ItineraryItem;
  locked: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const time = renderTimeRange(item.startsAt, item.endsAt);
  return (
    <article className="rounded-md border border-ink-200 bg-white p-3">
      {time && (
        <p className="text-[11px] uppercase tracking-wider text-ink-500">
          {time}
        </p>
      )}
      <p className="mt-0.5 text-sm font-medium leading-snug">{item.title}</p>
      {item.locationName && (
        <p className="mt-0.5 text-xs text-ink-500">{item.locationName}</p>
      )}
      {item.notes && (
        <p className="mt-1 whitespace-pre-line text-xs text-ink-500">
          {item.notes}
        </p>
      )}
      <div className="mt-3 flex items-center gap-2 border-t border-ink-100 pt-2 text-[11px]">
        <span className="text-ink-400">by @{item.createdBy.username}</span>
        <button
          type="button"
          onClick={onEdit}
          disabled={locked}
          className="ml-auto uppercase tracking-wider text-ink-500 hover:text-ink-950 disabled:cursor-default disabled:opacity-50 disabled:hover:text-ink-500"
        >
          Edit
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={locked}
          className="uppercase tracking-wider text-ink-400 hover:text-red-600 disabled:opacity-50 disabled:hover:text-ink-400"
        >
          Delete
        </button>
      </div>
    </article>
  );
}

// ---------------------------------------------------------------------------
// shared form
// ---------------------------------------------------------------------------

function ItineraryFormFields({
  form,
  setForm,
  error,
  busy,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  form: FormState;
  setForm: (next: FormState) => void;
  error: string | null;
  busy: boolean;
  submitLabel: string;
  onSubmit: (e: FormEvent) => void;
  onCancel: () => void;
}) {
  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm({ ...form, [key]: value });
  }
  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div>
        <label className="text-xs font-medium text-ink-600">Title</label>
        <Input
          className="mt-1"
          value={form.title}
          onChange={(e) => set("title", e.target.value)}
          placeholder="Visit the Colosseum"
          autoFocus
          required
        />
      </div>
      <div>
        <label className="text-xs font-medium text-ink-600">Notes</label>
        <Input
          className="mt-1"
          value={form.notes}
          onChange={(e) => set("notes", e.target.value)}
          placeholder="Optional"
        />
      </div>
      <div>
        <label className="text-xs font-medium text-ink-600">Location</label>
        <Input
          className="mt-1"
          value={form.locationName}
          onChange={(e) => set("locationName", e.target.value)}
          placeholder="Optional"
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Day">
          <Input
            type="number"
            min={0}
            value={form.dayIndex}
            onChange={(e) => set("dayIndex", e.target.value)}
            required
          />
        </Field>
        <Field label="Date">
          <input
            type="date"
            value={form.date}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              set("date", e.target.value)
            }
            className={controlClass}
          />
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Starts at">
          <input
            type="time"
            value={form.startsAt}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              set("startsAt", e.target.value)
            }
            className={controlClass}
          />
        </Field>
        <Field label="Ends at">
          <input
            type="time"
            value={form.endsAt}
            onChange={(e: ChangeEvent<HTMLInputElement>) =>
              set("endsAt", e.target.value)
            }
            className={controlClass}
          />
        </Field>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={busy}>
          {submitLabel}
        </Button>
        <button
          type="button"
          onClick={onCancel}
          disabled={busy}
          className="text-xs text-ink-500 hover:text-ink-950 disabled:opacity-50"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <label className="text-xs font-medium text-ink-600">{label}</label>
      <div className="mt-1">{children}</div>
    </div>
  );
}

const controlClass =
  "w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm";

// ---------------------------------------------------------------------------
// helpers — pure
// ---------------------------------------------------------------------------

type ValidatedForm =
  | { ok: true; values: { dayIndex: number; title: string } }
  | { ok: false; error: string };

function validateForm(form: FormState): ValidatedForm {
  const dayStr = form.dayIndex.trim();
  if (!/^\d+$/.test(dayStr)) {
    return { ok: false, error: "Day must be a non-negative integer." };
  }
  const dayIndex = parseInt(dayStr, 10);
  if (Number.isNaN(dayIndex) || dayIndex < 0) {
    return { ok: false, error: "Day must be a non-negative integer." };
  }

  const title = form.title.trim();
  if (title === "") {
    return { ok: false, error: "Title is required." };
  }

  if (form.date !== "" && !/^\d{4}-\d{2}-\d{2}$/.test(form.date)) {
    return { ok: false, error: "Date must be in YYYY-MM-DD format." };
  }
  if (form.startsAt !== "" && !isValidTimeInput(form.startsAt)) {
    return { ok: false, error: "Start time must be HH:MM." };
  }
  if (form.endsAt !== "" && !isValidTimeInput(form.endsAt)) {
    return { ok: false, error: "End time must be HH:MM." };
  }
  if (form.startsAt !== "" && form.endsAt !== "") {
    if (form.startsAt > form.endsAt) {
      return {
        ok: false,
        error: "Start time must be earlier than or equal to end time.",
      };
    }
  }

  return { ok: true, values: { dayIndex, title } };
}

function isValidTimeInput(s: string): boolean {
  return /^\d{2}:\d{2}(:\d{2})?$/.test(s);
}

function buildCreateBody(
  values: { dayIndex: number; title: string },
  form: FormState,
): Record<string, unknown> {
  const body: Record<string, unknown> = {
    dayIndex: values.dayIndex,
    title: values.title,
    notes: form.notes,
  };
  if (form.locationName.trim() !== "") {
    body.locationName = form.locationName.trim();
  }
  if (form.date !== "") body.date = form.date;
  if (form.startsAt !== "") body.startsAt = form.startsAt;
  if (form.endsAt !== "") body.endsAt = form.endsAt;
  return body;
}

function buildGroups(items: ItineraryItem[]): DayBucket[] {
  const buckets = new Map<number, ItineraryItem[]>();
  for (const it of items) {
    const list = buckets.get(it.dayIndex) ?? [];
    list.push(it);
    buckets.set(it.dayIndex, list);
  }
  const out: DayBucket[] = [];
  for (const [dayIndex, list] of buckets.entries()) {
    list.sort(compareItineraryItems);
    const date = list.find((i) => i.date)?.date;
    out.push({
      dayIndex,
      date: date ? formatDateDisplay(date) : undefined,
      items: list,
    });
  }
  out.sort((a, b) => a.dayIndex - b.dayIndex);
  return out;
}

// compareItineraryItems mirrors the backend's ORDER BY:
//   position asc -> startsAt asc nulls last -> createdAt asc -> id asc.
function compareItineraryItems(a: ItineraryItem, b: ItineraryItem): number {
  if (a.position !== b.position) return a.position - b.position;
  const sa = a.startsAt;
  const sb = b.startsAt;
  if (sa && sb) {
    if (sa !== sb) return sa < sb ? -1 : 1;
  } else if (sa) {
    return -1;
  } else if (sb) {
    return 1;
  }
  if (a.createdAt !== b.createdAt) {
    return a.createdAt < b.createdAt ? -1 : 1;
  }
  if (a.id !== b.id) return a.id < b.id ? -1 : 1;
  return 0;
}

// ---------------------------------------------------------------------------
// date / time formatting (no Date construction — avoid timezone shifts)
// ---------------------------------------------------------------------------

// formatDateDisplay turns the API's date value (YYYY-MM-DD or RFC3339
// midnight-UTC) into a stable YYYY-MM-DD string for display. Same trick
// PlanningBoard uses for task due dates.
function formatDateDisplay(raw: string): string {
  const m = raw.match(/^(\d{4}-\d{2}-\d{2})/);
  return m ? m[1] : raw;
}

// formatDateInput strips an RFC3339 prefix to YYYY-MM-DD so a <input
// type="date"> can use it as its initial value.
function formatDateInput(raw: string): string {
  return formatDateDisplay(raw);
}

// formatTimeInput shortens HH:MM:SS -> HH:MM for <input type="time">.
function formatTimeInput(raw: string): string {
  return raw.length >= 5 ? raw.slice(0, 5) : raw;
}

// renderTimeRange produces "14:30 – 16:00" / "from 14:30" / "until 16:00"
// for the item card's leading line, or null when neither time is set.
function renderTimeRange(
  startsAt: string | undefined,
  endsAt: string | undefined,
): string | null {
  const s = startsAt ? formatTimeInput(startsAt) : "";
  const e = endsAt ? formatTimeInput(endsAt) : "";
  if (s && e) return `${s} – ${e}`;
  if (s) return `from ${s}`;
  if (e) return `until ${e}`;
  return null;
}
