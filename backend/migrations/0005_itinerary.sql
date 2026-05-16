-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- M2 — itinerary, step 1 (schema only).
--
-- Adds the trip_itinerary_items table that backs a lightweight day-by-day
-- itinerary inside a trip. Backend handlers and the frontend land in later
-- commits; this migration only sets up storage and the indexes the listing
-- paths will need.
--
-- Decisions worth flagging:
--   * day_index is 0-based to match the rest of the project ("Day 0" =
--     arrival day in the user's terminology, with day 1, 2, ... as
--     subsequent days). Storing 0-based avoids an off-by-one between the
--     UI counter and the SQL index, and matches array-style buckets used
--     elsewhere in the planning code.
--   * date is optional because trips can be sketched as Day 0 / Day 1 /
--     Day 2 before exact calendar dates are known. The future API should
--     compute date from trips.starts_on + day_index when missing.
--   * starts_at / ends_at are time (not timestamptz) so they describe the
--     local time of day at the destination, not an absolute moment.
--   * title trim/non-empty is intentionally enforced in the service layer,
--     matching how trips.title and trip_tasks.title are validated. SQL
--     comment below documents the contract.
--   * position DEFAULT 0 is a DB-level fallback only. The future API
--     must compute max(position) + 1000 per (trip, day_index) bucket and
--     guard with a per-bucket advisory lock — same pattern trip_tasks
--     uses since 0004 + the M2 race-safety fix.
--   * created_by_user_id ON DELETE RESTRICT preserves authorship. A user
--     cannot be deleted while they still own itinerary entries; this
--     matches trip_tasks.
-- ===========================================================================

CREATE TABLE trip_itinerary_items (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id            uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,

    -- 0-based day bucket. day 0 is the trip's first day. Service layer
    -- decides what "day 0" means for a given trip (typically arrival).
    day_index          integer NOT NULL CHECK (day_index >= 0),

    -- Optional concrete calendar date. Allowed to be null while the trip
    -- is still being planned in relative-day mode.
    date               date,

    -- title is required at the column level but service layer is the
    -- source of truth for "non-empty after trim" — same pattern as
    -- trips.title and trip_tasks.title.
    title              text NOT NULL,
    notes              text NOT NULL DEFAULT '',
    location_name      text,

    -- local time-of-day at the destination, not an absolute moment.
    starts_at          time,
    ends_at            time,

    -- Service must assign max(position) + 1000 per (trip, day_index);
    -- DEFAULT 0 is a DB fallback only.
    position           integer NOT NULL DEFAULT 0 CHECK (position >= 0),

    created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- Primary listing path: day-by-day view (group by day_index, ordered
-- within each day by position). Single composite serves both filter and sort.
CREATE INDEX trip_itinerary_items_trip_day_position_idx
    ON trip_itinerary_items (trip_id, day_index, position);

-- Calendar-style filtering when explicit dates are set. Partial index
-- keeps it small while date adoption is sparse.
CREATE INDEX trip_itinerary_items_trip_date_idx
    ON trip_itinerary_items (trip_id, date)
    WHERE date IS NOT NULL;

-- Activity-style sort and pagination by recency.
CREATE INDEX trip_itinerary_items_trip_created_idx
    ON trip_itinerary_items (trip_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS trip_itinerary_items_trip_created_idx;
DROP INDEX IF EXISTS trip_itinerary_items_trip_date_idx;
DROP INDEX IF EXISTS trip_itinerary_items_trip_day_position_idx;
DROP TABLE IF EXISTS trip_itinerary_items;

-- +goose StatementEnd
