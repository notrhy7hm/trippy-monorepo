import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "./ui/Button";
import { Card } from "./ui/Card";
import { Input } from "./ui/Input";
import { api, ApiError } from "../lib/api";

export type BudgetMember = {
  userId: string;
  username: string;
  displayName: string;
};

type Split = {
  userId: string;
  shareCents: number;
};

type Expense = {
  id: string;
  title: string;
  amountCents: number;
  currency: string;
  category: string;
  paidByUserId: string;
  expenseDate?: string;
  notes: string;
  splits: Split[];
};

type MemberBalance = {
  userId: string;
  username: string;
  displayName: string;
  paidCents: number;
  shareCents: number;
  balanceCents: number;
};

type Settlement = {
  debtorUserId: string;
  creditorUserId: string;
  amountCents: number;
};

type CurrencySummary = {
  currency: string;
  totalCents: number;
  members: MemberBalance[];
  settlements: Settlement[];
};

type Summary = {
  currencies: CurrencySummary[];
};

type FormState = {
  title: string;
  amount: string;
  currency: string;
  category: string;
  paidByUserId: string;
  expenseDate: string;
  notes: string;
  shares: Record<string, string>;
};

export function BudgetBoard({
  tripSlug,
  members,
  viewerIsMember,
  canManage,
  currentUserId,
  reloadTrip,
}: {
  tripSlug: string | undefined;
  members: BudgetMember[];
  viewerIsMember: boolean;
  canManage: boolean;
  currentUserId?: string;
  reloadTrip: () => Promise<void>;
}) {
  const [expenses, setExpenses] = useState<Expense[] | null>(null);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Expense | null>(null);
  const [form, setForm] = useState<FormState>(() =>
    blankForm(members, currentUserId),
  );
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const memberById = useMemo(() => {
    const out = new Map<string, BudgetMember>();
    for (const m of members) out.set(m.userId, m);
    return out;
  }, [members]);

  const refresh = useCallback(async () => {
    if (!tripSlug || !viewerIsMember) return;
    try {
      const [nextExpenses, nextSummary] = await Promise.all([
        api<Expense[]>("GET", `/trips/${tripSlug}/expenses`),
        api<Summary>("GET", `/trips/${tripSlug}/budget/summary`),
      ]);
      setExpenses(nextExpenses);
      setSummary(nextSummary);
      setLoadError(null);
    } catch (err) {
      setExpenses((prev) => prev ?? []);
      setSummary((prev) => prev ?? { currencies: [] });
      setLoadError(
        err instanceof ApiError ? err.message : "Failed to load budget",
      );
    }
  }, [tripSlug, viewerIsMember]);

  useEffect(() => {
    setExpenses(null);
    setSummary(null);
    setLoadError(null);
    setActionError(null);
    setShowForm(false);
    setEditing(null);
    setForm(blankForm(members, currentUserId));
    setFormError(null);
    setBusy(false);
    if (viewerIsMember && tripSlug) refresh();
  }, [tripSlug, viewerIsMember, refresh, members, currentUserId]);

  function openCreate() {
    if (!canManage) return;
    setEditing(null);
    setForm(blankForm(members, currentUserId));
    setFormError(null);
    setShowForm(true);
  }

  function openEdit(expense: Expense) {
    if (!canManage) return;
    setEditing(expense);
    setForm(formFromExpense(expense, members));
    setFormError(null);
    setShowForm(true);
  }

  function closeForm() {
    setShowForm(false);
    setEditing(null);
    setFormError(null);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!tripSlug || busy || !canManage) return;

    const built = buildPayload(form, members);
    if (!built.ok) {
      setFormError(built.error);
      return;
    }

    setBusy(true);
    setFormError(null);
    setActionError(null);
    try {
      if (editing) {
        await api("PATCH", `/trips/${tripSlug}/expenses/${editing.id}`, built.body);
      } else {
        await api("POST", `/trips/${tripSlug}/expenses`, built.body);
      }
      closeForm();
      await refresh();
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
        setActionError(err.message);
        await reloadTrip();
        await refresh();
      } else {
        setFormError(err instanceof ApiError ? err.message : "Save failed");
      }
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(expense: Expense) {
    if (!tripSlug || busy || !canManage) return;
    if (!window.confirm(`Delete "${expense.title}"?`)) return;
    setBusy(true);
    setActionError(null);
    try {
      await api("DELETE", `/trips/${tripSlug}/expenses/${expense.id}`);
      if (editing?.id === expense.id) closeForm();
      await refresh();
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
        setActionError(err.message);
        await reloadTrip();
        await refresh();
      } else {
        setActionError(err instanceof ApiError ? err.message : "Delete failed");
      }
    } finally {
      setBusy(false);
    }
  }

  function setField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function splitEqually() {
    const amount = parseCents(form.amount);
    if (amount === null || amount <= 0) {
      setFormError("Enter a valid positive amount first.");
      return;
    }
    setForm((prev) => ({ ...prev, shares: equalShares(members, amount) }));
    setFormError(null);
  }

  return (
    <section className="mt-10">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-baseline sm:justify-between">
        <div>
          <h2 className="text-lg font-medium">Budget</h2>
          <p className="text-xs text-ink-500">
            Track expenses, splits, balances, and suggested settlements.
          </p>
        </div>
        {viewerIsMember && canManage && (
          <Button
            variant={showForm ? "ghost" : "primary"}
            onClick={() => (showForm ? closeForm() : openCreate())}
            disabled={busy}
            className="w-full sm:w-auto"
          >
            {showForm ? "Cancel" : "Add expense"}
          </Button>
        )}
      </div>

      {!viewerIsMember ? (
        <Card className="mt-4 p-6 text-center">
          <p className="text-sm text-ink-500">
            Only trip members can see the budget.
          </p>
        </Card>
      ) : (
        <>
          {!canManage && (
            <p className="mt-3 text-xs text-ink-500">
              You can view the budget. Budget manager, admin, or owner role is
              required to change expenses.
            </p>
          )}

          {loadError && (
            <p className="mt-4 text-xs text-ink-500">
              Budget is temporarily unavailable.
            </p>
          )}
          {actionError && (
            <p className="mt-4 text-sm text-red-600">{actionError}</p>
          )}

          {showForm && (
            <Card className="mt-4 p-4 sm:p-5">
              <form onSubmit={onSubmit} className="space-y-4">
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <label className="block">
                    <span className="text-xs font-medium text-ink-600">
                      Title
                    </span>
                    <Input
                      className="mt-1"
                      value={form.title}
                      onChange={(e) => setField("title", e.target.value)}
                      placeholder="Hotel"
                      required
                    />
                  </label>
                  <label className="block">
                    <span className="text-xs font-medium text-ink-600">
                      Amount
                    </span>
                    <Input
                      className="mt-1"
                      inputMode="decimal"
                      value={form.amount}
                      onChange={(e) => setField("amount", e.target.value)}
                      placeholder="120.00"
                      required
                    />
                  </label>
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                  <label className="block">
                    <span className="text-xs font-medium text-ink-600">
                      Currency
                    </span>
                    <Input
                      className="mt-1 uppercase"
                      value={form.currency}
                      onChange={(e) =>
                        setField("currency", e.target.value.toUpperCase())
                      }
                      maxLength={3}
                    />
                  </label>
                  <label className="block">
                    <span className="text-xs font-medium text-ink-600">
                      Category
                    </span>
                    <Input
                      className="mt-1"
                      value={form.category}
                      onChange={(e) => setField("category", e.target.value)}
                      placeholder="lodging"
                    />
                  </label>
                  <label className="block">
                    <span className="text-xs font-medium text-ink-600">
                      Date
                    </span>
                    <input
                      type="date"
                      value={form.expenseDate}
                      onChange={(e) => setField("expenseDate", e.target.value)}
                      className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
                    />
                  </label>
                </div>

                <label className="block">
                  <span className="text-xs font-medium text-ink-600">
                    Paid by
                  </span>
                  <select
                    value={form.paidByUserId}
                    onChange={(e) => setField("paidByUserId", e.target.value)}
                    className="mt-1 w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm"
                    required
                  >
                    {members.map((m) => (
                      <option key={m.userId} value={m.userId}>
                        {m.displayName || m.username} (@{m.username})
                      </option>
                    ))}
                  </select>
                </label>

                <label className="block">
                  <span className="text-xs font-medium text-ink-600">
                    Notes
                  </span>
                  <Input
                    className="mt-1"
                    value={form.notes}
                    onChange={(e) => setField("notes", e.target.value)}
                    placeholder="Optional"
                  />
                </label>

                <div>
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <span className="text-xs font-medium text-ink-600">
                      Split shares
                    </span>
                    <button
                      type="button"
                      onClick={splitEqually}
                      className="text-left text-xs text-ink-500 hover:text-ink-950 sm:text-right"
                    >
                      Split equally
                    </button>
                  </div>
                  <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-2">
                    {members.map((m) => (
                      <label key={m.userId} className="block">
                        <span className="text-[11px] text-ink-500">
                          @{m.username}
                        </span>
                        <Input
                          className="mt-1"
                          inputMode="decimal"
                          value={form.shares[m.userId] ?? ""}
                          onChange={(e) =>
                            setForm((prev) => ({
                              ...prev,
                              shares: {
                                ...prev.shares,
                                [m.userId]: e.target.value,
                              },
                            }))
                          }
                          placeholder="0.00"
                        />
                      </label>
                    ))}
                  </div>
                </div>

                {formError && (
                  <p className="text-sm text-red-600">{formError}</p>
                )}
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Button type="submit" disabled={busy} className="w-full sm:w-auto">
                    {busy ? "Saving…" : editing ? "Save expense" : "Add expense"}
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={closeForm}
                    disabled={busy}
                    className="w-full sm:w-auto"
                  >
                    Cancel
                  </Button>
                </div>
              </form>
            </Card>
          )}

          <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
            <Card className="p-4 sm:p-5">
              <h3 className="text-sm font-medium">Expenses</h3>
              {expenses === null ? (
                <p className="mt-3 text-sm text-ink-500">Loading…</p>
              ) : expenses.length === 0 ? (
                <p className="mt-3 text-sm text-ink-500">
                  No expenses yet.
                </p>
              ) : (
                <ul className="mt-3 divide-y divide-ink-100">
                  {expenses.map((expense) => (
                    <ExpenseRow
                      key={expense.id}
                      expense={expense}
                      memberById={memberById}
                      canManage={canManage}
                      busy={busy}
                      onEdit={() => openEdit(expense)}
                      onDelete={() => onDelete(expense)}
                    />
                  ))}
                </ul>
              )}
            </Card>

            <Card className="p-4 sm:p-5">
              <h3 className="text-sm font-medium">Summary</h3>
              {summary === null ? (
                <p className="mt-3 text-sm text-ink-500">Loading…</p>
              ) : summary.currencies.length === 0 ? (
                <p className="mt-3 text-sm text-ink-500">
                  Add an expense to see balances.
                </p>
              ) : (
                <div className="mt-3 space-y-5">
                  {summary.currencies.map((cur) => (
                    <CurrencySummaryCard
                      key={cur.currency}
                      summary={cur}
                      memberById={memberById}
                    />
                  ))}
                </div>
              )}
            </Card>
          </div>
        </>
      )}
    </section>
  );
}

function ExpenseRow({
  expense,
  memberById,
  canManage,
  busy,
  onEdit,
  onDelete,
}: {
  expense: Expense;
  memberById: Map<string, BudgetMember>;
  canManage: boolean;
  busy: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const payer = labelFor(memberById, expense.paidByUserId);
  return (
    <li className="py-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="break-words text-sm font-medium">{expense.title}</p>
          <p className="mt-1 text-xs text-ink-500">
            {formatMoney(expense.amountCents, expense.currency)} · paid by{" "}
            {payer}
            {expense.expenseDate ? ` · ${expense.expenseDate}` : ""}
          </p>
          {expense.notes && (
            <p className="mt-1 break-words text-xs text-ink-500">
              {expense.notes}
            </p>
          )}
          <p className="mt-1 text-[11px] text-ink-400">
            Split:{" "}
            {expense.splits
              .map(
                (split) =>
                  `${labelFor(memberById, split.userId)} ${formatMoney(
                    split.shareCents,
                    expense.currency,
                  )}`,
              )
              .join(" · ")}
          </p>
        </div>
        {canManage && (
          <div className="flex shrink-0 gap-2">
            <button
              type="button"
              onClick={onEdit}
              disabled={busy}
              className="text-xs text-ink-500 hover:text-ink-950 disabled:opacity-50"
            >
              Edit
            </button>
            <button
              type="button"
              onClick={onDelete}
              disabled={busy}
              className="text-xs text-ink-400 hover:text-red-600 disabled:opacity-50"
            >
              Delete
            </button>
          </div>
        )}
      </div>
    </li>
  );
}

function CurrencySummaryCard({
  summary,
  memberById,
}: {
  summary: CurrencySummary;
  memberById: Map<string, BudgetMember>;
}) {
  return (
    <div>
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-medium uppercase tracking-wider text-ink-500">
          {summary.currency}
        </p>
        <p className="text-sm font-medium">
          {formatMoney(summary.totalCents, summary.currency)}
        </p>
      </div>
      <ul className="mt-2 space-y-1">
        {summary.members.map((m) => (
          <li
            key={m.userId}
            className="flex items-center justify-between gap-3 text-xs"
          >
            <span className="min-w-0 truncate">
              {m.displayName || m.username}
            </span>
            <span
              className={
                m.balanceCents >= 0 ? "text-emerald-700" : "text-red-600"
              }
            >
              {formatSignedMoney(m.balanceCents, summary.currency)}
            </span>
          </li>
        ))}
      </ul>
      {summary.settlements.length > 0 && (
        <div className="mt-3 rounded-md bg-ink-50 p-3">
          <p className="text-[11px] font-medium uppercase tracking-wider text-ink-500">
            Settle up
          </p>
          <ul className="mt-2 space-y-1">
            {summary.settlements.map((s) => (
              <li
                key={`${s.debtorUserId}-${s.creditorUserId}-${s.amountCents}`}
                className="text-xs text-ink-600"
              >
                {labelFor(memberById, s.debtorUserId)} pays{" "}
                {labelFor(memberById, s.creditorUserId)}{" "}
                {formatMoney(s.amountCents, summary.currency)}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function blankForm(members: BudgetMember[], currentUserId?: string): FormState {
  const payer =
    currentUserId && members.some((m) => m.userId === currentUserId)
      ? currentUserId
      : members[0]?.userId ?? "";
  return {
    title: "",
    amount: "",
    currency: "EUR",
    category: "other",
    paidByUserId: payer,
    expenseDate: "",
    notes: "",
    shares: Object.fromEntries(members.map((m) => [m.userId, "0.00"])),
  };
}

function formFromExpense(expense: Expense, members: BudgetMember[]): FormState {
  const shares = Object.fromEntries(members.map((m) => [m.userId, "0.00"]));
  for (const split of expense.splits) {
    shares[split.userId] = centsToInput(split.shareCents);
  }
  return {
    title: expense.title,
    amount: centsToInput(expense.amountCents),
    currency: expense.currency,
    category: expense.category,
    paidByUserId: expense.paidByUserId,
    expenseDate: expense.expenseDate ?? "",
    notes: expense.notes,
    shares,
  };
}

function buildPayload(
  form: FormState,
  members: BudgetMember[],
):
  | { ok: true; body: Record<string, unknown> }
  | { ok: false; error: string } {
  const title = form.title.trim();
  if (title === "") return { ok: false, error: "Title is required." };

  const amountCents = parseCents(form.amount);
  if (amountCents === null || amountCents <= 0) {
    return { ok: false, error: "Enter a valid positive amount." };
  }

  const currency = form.currency.trim().toUpperCase();
  if (!/^[A-Z]{3}$/.test(currency)) {
    return { ok: false, error: "Currency must be a 3-letter code." };
  }

  if (!members.some((m) => m.userId === form.paidByUserId)) {
    return { ok: false, error: "Choose who paid." };
  }

  let splits = members.map((m) => {
    const share = parseCents(form.shares[m.userId] ?? "0");
    return { userId: m.userId, shareCents: share };
  });
  if (splits.some((s) => s.shareCents === null)) {
    return { ok: false, error: "Split shares must be valid amounts." };
  }
  if (splits.every((s) => s.shareCents === 0)) {
    splits = Object.entries(equalShares(members, amountCents)).map(
      ([userId, share]) => ({
        userId,
        shareCents: parseCents(share),
      }),
    );
  }

  const finalSplits = splits.map((s) => ({
    userId: s.userId,
    shareCents: s.shareCents ?? 0,
  }));
  const splitTotal = finalSplits.reduce((sum, s) => sum + s.shareCents, 0);
  if (splitTotal !== amountCents) {
    return {
      ok: false,
      error: "Split shares must add up exactly to the expense amount.",
    };
  }

  return {
    ok: true,
    body: {
      title,
      amountCents,
      currency,
      category: form.category.trim() || "other",
      paidByUserId: form.paidByUserId,
      expenseDate: form.expenseDate,
      notes: form.notes,
      splits: finalSplits,
    },
  };
}

function equalShares(members: BudgetMember[], totalCents: number) {
  const out: Record<string, string> = {};
  if (members.length === 0) return out;
  const base = Math.floor(totalCents / members.length);
  let remainder = totalCents % members.length;
  for (const m of members) {
    const extra = remainder > 0 ? 1 : 0;
    out[m.userId] = centsToInput(base + extra);
    remainder -= extra;
  }
  return out;
}

function parseCents(raw: string): number | null {
  const s = raw.trim();
  if (!/^\d+(\.\d{1,2})?$/.test(s)) return null;
  const [whole, frac = ""] = s.split(".");
  return Number(whole) * 100 + Number(frac.padEnd(2, "0"));
}

function centsToInput(cents: number) {
  return (cents / 100).toFixed(2);
}

function formatMoney(cents: number, currency: string) {
  return `${currency} ${centsToInput(cents)}`;
}

function formatSignedMoney(cents: number, currency: string) {
  const sign = cents > 0 ? "+" : cents < 0 ? "-" : "";
  return `${sign}${formatMoney(Math.abs(cents), currency)}`;
}

function labelFor(memberById: Map<string, BudgetMember>, userId: string) {
  const member = memberById.get(userId);
  return member ? `@${member.username}` : "unknown member";
}
