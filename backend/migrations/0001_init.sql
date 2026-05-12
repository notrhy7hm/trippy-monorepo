-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- users + profile -----------------------------------------------------------

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

CREATE TABLE user_profiles (
    user_id      uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name text NOT NULL DEFAULT '',
    bio          text NOT NULL DEFAULT '',
    avatar_url   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- trips ---------------------------------------------------------------------

CREATE TYPE trip_visibility AS ENUM ('private', 'friends', 'public');
CREATE TYPE trip_role       AS ENUM ('owner', 'admin', 'member');

CREATE TABLE trips (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text NOT NULL UNIQUE,
    owner_id    uuid NOT NULL REFERENCES users(id),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    starts_on   date,
    ends_on     date,
    visibility  trip_visibility NOT NULL DEFAULT 'private',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE INDEX trips_owner_idx     ON trips(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX trips_starts_on_idx ON trips(starts_on) WHERE deleted_at IS NULL;

CREATE TABLE trip_members (
    trip_id   uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      trip_role NOT NULL DEFAULT 'member',
    tags      text[] NOT NULL DEFAULT ARRAY[]::text[],
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (trip_id, user_id)
);

CREATE INDEX trip_members_user_idx ON trip_members(user_id);

-- placeholders for later milestones (created now so foreign keys work later) -

CREATE TABLE trip_invites (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id          uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    invitee_user_id  uuid REFERENCES users(id),
    invitee_email    text,
    token            text NOT NULL UNIQUE,
    status           text NOT NULL DEFAULT 'pending',
    expires_at       timestamptz NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trip_favorites (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id    uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, trip_id)
);

CREATE TABLE audit_log (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     uuid REFERENCES users(id),
    trip_id      uuid REFERENCES trips(id),
    action       text NOT NULL,
    target_type  text NOT NULL,
    target_id    uuid,
    payload      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_trip_idx  ON audit_log(trip_id, created_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log(actor_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS trip_favorites;
DROP TABLE IF EXISTS trip_invites;
DROP TABLE IF EXISTS trip_members;
DROP TABLE IF EXISTS trips;
DROP TYPE  IF EXISTS trip_role;
DROP TYPE  IF EXISTS trip_visibility;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
