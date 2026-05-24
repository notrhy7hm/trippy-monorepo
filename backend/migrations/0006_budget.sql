-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- M3 — budget, step 1 (schema only).
--
-- Adds the two tables that back per-trip expense tracking:
--   * trip_expenses        — one row per recorded expense (the "what was
--                            paid, and by whom" record).
--   * trip_expense_splits  — one row per participant in an expense (the
--                            "who owes their share" record).
--
-- Backend handlers and the frontend land in later commits; this migration
-- only sets up storage, constraints, and the indexes the listing /
-- balance-summary paths will need.
--
-- Decisions worth flagging:
--   * Money is stored as integer cents (amount_cents / share_cents, both
--     bigint) — never floating point. Float math accumulates rounding
--     error across splits and sums; integer cents is exact, and bigint
--     comfortably holds any realistic trip total. All split arithmetic
--     ("who owes whom") is therefore exact integer arithmetic.
--   * currency is stored per expense (not per trip) so a single trip can
--     mix receipts paid in different countries. A CHECK pins it to a
--     3-letter uppercase code (ISO-4217 *shape* only — the DB does not
--     verify the code is a real currency). Currency conversion is
--     explicitly out of scope for M3.
--   * category is free-form text with DEFAULT 'other'. Left unconstrained
--     for now; a curated set can become an ENUM in a later milestone
--     without blocking M3.
--   * title trim / non-empty is intentionally enforced in the service
--     layer, matching trips.title, trip_tasks.title and
--     trip_itinerary_items.title. The DB only enforces NOT NULL here.
--   * paid_by_user_id and created_by_user_id use ON DELETE RESTRICT so a
--     user cannot be deleted while expense history still references them
--     — the same authorship-preserving choice trip_tasks and
--     trip_itinerary_items make. trip_id and expense_id use ON DELETE
--     CASCADE so deleting a trip (or an expense) cleans up its children.
--   * Cross-row / cross-table invariants are NOT enforced here:
--       - paid_by_user_id and every split user_id must be a trip member;
--       - SUM(share_cents) for an expense must equal its amount_cents.
--     Both depend on trip membership and span multiple rows, so they are
--     service-layer checks — same pattern as trip_tasks assignee
--     membership validation.
-- ===========================================================================

CREATE TABLE trip_expenses (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id            uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    title              text NOT NULL,

    -- Total cost in integer cents; always strictly positive.
    amount_cents       bigint NOT NULL CHECK (amount_cents > 0),

    -- ISO-4217-shaped 3-letter uppercase code. Shape only — not checked
    -- against a real currency list at the DB level.
    currency           text NOT NULL DEFAULT 'EUR'
                             CHECK (currency ~ '^[A-Z]{3}$'),

    category           text NOT NULL DEFAULT 'other',
    paid_by_user_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- Optional concrete date the expense occurred; may be null while a
    -- trip is still being planned in relative terms.
    expense_date       date,

    notes              text NOT NULL DEFAULT '',
    created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trip_expense_splits (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id  uuid NOT NULL REFERENCES trip_expenses(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- This user's share of the expense, in integer cents. >= 0 (a
    -- zero-share participant is allowed); the
    -- SUM(share_cents) == expense.amount_cents invariant is a
    -- service-layer check.
    share_cents bigint NOT NULL CHECK (share_cents >= 0),

    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    -- One split row per user per expense.
    UNIQUE (expense_id, user_id)
);

-- trip_expenses listing paths ------------------------------------------------
-- Primary per-trip expense list; commonly ordered / filtered by date.
CREATE INDEX trip_expenses_trip_date_idx
    ON trip_expenses (trip_id, expense_date);

-- Per-trip filtering by category (spend-breakdown view).
CREATE INDEX trip_expenses_trip_category_idx
    ON trip_expenses (trip_id, category);

-- "What did user X pay for on this trip" — feeds the who-paid side of the
-- later balance summary.
CREATE INDEX trip_expenses_trip_paid_by_idx
    ON trip_expenses (trip_id, paid_by_user_id);

-- trip_expense_splits lookup paths -------------------------------------------
-- Per-user split lookup — feeds the who-owes side of the balance summary.
CREATE INDEX trip_expense_splits_user_idx
    ON trip_expense_splits (user_id);

-- No separate index on (expense_id): the UNIQUE (expense_id, user_id)
-- constraint above already builds a btree whose leftmost column is
-- expense_id, so "all splits for one expense" lookups are covered without
-- a redundant index.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop child table before parent (FK direction: splits -> expenses).
DROP INDEX IF EXISTS trip_expense_splits_user_idx;
DROP TABLE IF EXISTS trip_expense_splits;

DROP INDEX IF EXISTS trip_expenses_trip_paid_by_idx;
DROP INDEX IF EXISTS trip_expenses_trip_category_idx;
DROP INDEX IF EXISTS trip_expenses_trip_date_idx;
DROP TABLE IF EXISTS trip_expenses;

-- +goose StatementEnd
