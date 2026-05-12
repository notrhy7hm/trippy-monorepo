-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- M1 — collaboration core schema.
--
-- This migration only adds tables, columns, and constraints. No backend code
-- or product behaviour changes here; later M1 commits wire the schema up.
--
-- Adds:
--   * Extended trip_role enum: planner, budget_manager, viewer (M1 treats
--     planner/budget_manager as 'member' and viewer as read-only; the wider
--     enum is ready for kanban/timeline/budget milestones).
--   * Single-owner invariant on trip_members.
--   * friendships (canonical pair, user_a < user_b).
--   * friend_requests (pull-only state machine; no notifications in M1).
--   * trip_invites extended with invited_by_user_id, role, responded_at,
--     a status CHECK, a target CHECK, and partial-unique guards against
--     duplicate pending invites per (trip, user) and (trip, email).
--
-- Ownership transfer is intentionally out of scope for M1 — see TODO below.
-- ===========================================================================

-- ----- trip_role: extend by recreating the type ----------------------------
-- ALTER TYPE ... ADD VALUE cannot run inside a transaction. Recreating keeps
-- the migration transactional and the column type strict.
ALTER TABLE trip_members ALTER COLUMN role DROP DEFAULT;
ALTER TABLE trip_members ALTER COLUMN role TYPE text USING role::text;
DROP TYPE trip_role;
CREATE TYPE trip_role AS ENUM (
    'owner',
    'admin',
    'planner',
    'budget_manager',
    'member',
    'viewer'
);
ALTER TABLE trip_members ALTER COLUMN role TYPE trip_role USING role::trip_role;
ALTER TABLE trip_members ALTER COLUMN role SET DEFAULT 'member';

-- ----- exactly one owner per trip ------------------------------------------
-- TODO(milestone-later): owner transfer. For M1, the service layer must
-- forbid owner removal/demotion and creation already inserts a single
-- 'owner' row, so the existing data does not violate this index.
CREATE UNIQUE INDEX trip_members_one_owner_idx
    ON trip_members(trip_id)
    WHERE role = 'owner';

-- ----- friendships ---------------------------------------------------------
-- Canonical pair: user_a < user_b ensures one row per friendship regardless
-- of who initiated. Reads use OR on both columns.
CREATE TABLE friendships (
    user_a     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_a, user_b),
    CHECK (user_a < user_b)
);

CREATE INDEX friendships_user_b_idx ON friendships(user_b);

-- ----- friend_requests -----------------------------------------------------
CREATE TABLE friend_requests (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    from_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       text NOT NULL DEFAULT 'pending',
    message      text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    responded_at timestamptz,
    CHECK (from_user_id <> to_user_id),
    CHECK (status IN ('pending','accepted','declined','canceled'))
);

-- At most one pending request between any pair, regardless of direction.
CREATE UNIQUE INDEX friend_requests_pending_pair_idx
    ON friend_requests (LEAST(from_user_id, to_user_id), GREATEST(from_user_id, to_user_id))
    WHERE status = 'pending';

CREATE INDEX friend_requests_to_pending_idx
    ON friend_requests (to_user_id, created_at DESC)
    WHERE status = 'pending';

CREATE INDEX friend_requests_from_pending_idx
    ON friend_requests (from_user_id, created_at DESC)
    WHERE status = 'pending';

-- ----- trip_invites: extend the M0 table -----------------------------------
ALTER TABLE trip_invites
    ADD COLUMN invited_by_user_id uuid REFERENCES users(id),
    ADD COLUMN role               trip_role NOT NULL DEFAULT 'member',
    ADD COLUMN responded_at       timestamptz;

ALTER TABLE trip_invites
    ADD CONSTRAINT trip_invites_status_check
        CHECK (status IN ('pending','accepted','declined','revoked','expired')),
    ADD CONSTRAINT trip_invites_target_check
        CHECK (invitee_user_id IS NOT NULL OR invitee_email IS NOT NULL);

CREATE UNIQUE INDEX trip_invites_pending_user_idx
    ON trip_invites (trip_id, invitee_user_id)
    WHERE status = 'pending' AND invitee_user_id IS NOT NULL;

CREATE UNIQUE INDEX trip_invites_pending_email_idx
    ON trip_invites (trip_id, lower(invitee_email))
    WHERE status = 'pending' AND invitee_email IS NOT NULL;

CREATE INDEX trip_invites_invitee_pending_idx
    ON trip_invites (invitee_user_id, status)
    WHERE invitee_user_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- ----- trip_invites: revert extensions -------------------------------------
DROP INDEX IF EXISTS trip_invites_invitee_pending_idx;
DROP INDEX IF EXISTS trip_invites_pending_email_idx;
DROP INDEX IF EXISTS trip_invites_pending_user_idx;

ALTER TABLE trip_invites
    DROP CONSTRAINT IF EXISTS trip_invites_target_check,
    DROP CONSTRAINT IF EXISTS trip_invites_status_check;

ALTER TABLE trip_invites
    DROP COLUMN IF EXISTS responded_at,
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS invited_by_user_id;

-- ----- friend_requests / friendships ---------------------------------------
DROP TABLE IF EXISTS friend_requests;
DROP TABLE IF EXISTS friendships;

-- ----- one-owner invariant -------------------------------------------------
DROP INDEX IF EXISTS trip_members_one_owner_idx;

-- ----- restore trip_role to the original three values ----------------------
-- Map any new role back to 'member' so the narrower enum cast succeeds. This
-- is intentionally lossy: rolling back this migration downgrades planner /
-- budget_manager / viewer rows to plain members.
UPDATE trip_members
   SET role = 'member'
 WHERE role NOT IN ('owner','admin','member');

ALTER TABLE trip_members ALTER COLUMN role DROP DEFAULT;
ALTER TABLE trip_members ALTER COLUMN role TYPE text USING role::text;
DROP TYPE trip_role;
CREATE TYPE trip_role AS ENUM ('owner','admin','member');
ALTER TABLE trip_members ALTER COLUMN role TYPE trip_role USING role::trip_role;
ALTER TABLE trip_members ALTER COLUMN role SET DEFAULT 'member';

-- +goose StatementEnd
