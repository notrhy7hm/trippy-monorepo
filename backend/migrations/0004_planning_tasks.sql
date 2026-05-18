-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- M2 — planning workspace, step 1 (schema only).
--
-- Adds the trip_tasks table that backs the per-trip planning board.
-- Backend handlers and the frontend land in later commits; this migration
-- only sets up storage and the indexes the listing/sorting paths will need.
--
-- Decisions worth flagging:
--   * Status and priority are Postgres ENUMs (matching trip_role / trip_visibility
--     in 0001) rather than CHECK-constrained text columns. They are tightly
--     bounded sets that the service layer also pins; ENUMs give us the strictest
--     validation at the storage layer for the same effort.
--   * Title trim/non-empty is intentionally validated in the service, not the
--     DB. The existing project style only enforces simple shape at the DB level;
--     "non-empty after trim" requires a CHECK with btrim which works but is
--     inconsistent with how trips.title and users.username are validated.
--   * Assignee membership is NOT enforced at the DB. The future API will
--     verify the assignee is a trip member before write; falling back to
--     `ON DELETE SET NULL` for the FK means removing a user nulls out their
--     assignments instead of corrupting the row.
--   * `position` is `integer NOT NULL DEFAULT 0` for now. Re-ordering will
--     land with the planning API; this column reserves the field so reads
--     can order deterministically by (status, position) from day one.
-- ===========================================================================

CREATE TYPE task_status   AS ENUM ('todo', 'in_progress', 'done');
CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high');

CREATE TABLE trip_tasks (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id             uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    title               text NOT NULL,
    description         text NOT NULL DEFAULT '',
    status              task_status   NOT NULL DEFAULT 'todo',
    priority            task_priority NOT NULL DEFAULT 'normal',
    assignee_user_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    position            integer NOT NULL DEFAULT 0,
    due_date            date,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- Primary listing path: board view (group by status, ordered within each
-- column by position). Single composite index serves both filter and sort.
CREATE INDEX trip_tasks_trip_status_position_idx
    ON trip_tasks (trip_id, status, position);

-- "My tasks" / per-assignee filtering inside a trip.
CREATE INDEX trip_tasks_trip_assignee_idx
    ON trip_tasks (trip_id, assignee_user_id)
    WHERE assignee_user_id IS NOT NULL;

-- Activity-style sort and pagination by recency.
CREATE INDEX trip_tasks_trip_created_idx
    ON trip_tasks (trip_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS trip_tasks_trip_created_idx;
DROP INDEX IF EXISTS trip_tasks_trip_assignee_idx;
DROP INDEX IF EXISTS trip_tasks_trip_status_position_idx;
DROP TABLE IF EXISTS trip_tasks;
DROP TYPE  IF EXISTS task_priority;
DROP TYPE  IF EXISTS task_status;

-- +goose StatementEnd
