-- +goose Up
-- +goose StatementBegin
CREATE TABLE trip_agents (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id      uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    capabilities text[] NOT NULL DEFAULT ARRAY[]::text[],
    config       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_id)
);

CREATE TABLE agent_messages (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   uuid NOT NULL REFERENCES trip_agents(id) ON DELETE CASCADE,
    author_id  uuid REFERENCES users(id),
    role       text NOT NULL,
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX agent_messages_agent_idx ON agent_messages(agent_id, created_at);

CREATE TABLE agent_action_proposals (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id               uuid NOT NULL REFERENCES trip_agents(id) ON DELETE CASCADE,
    trip_id                uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    proposed_by_message_id uuid REFERENCES agent_messages(id),
    tool                   text NOT NULL,
    payload                jsonb NOT NULL DEFAULT '{}'::jsonb,
    status                 text NOT NULL DEFAULT 'pending',
    confirmed_by_user_id   uuid REFERENCES users(id),
    confirmed_at           timestamptz,
    executed_at            timestamptz,
    result                 jsonb,
    created_at             timestamptz NOT NULL DEFAULT now(),
    CHECK (status IN ('pending', 'approved', 'rejected', 'executed', 'failed'))
);

CREATE INDEX agent_proposals_trip_idx   ON agent_action_proposals(trip_id, status, created_at DESC);
CREATE INDEX agent_proposals_status_idx ON agent_action_proposals(status, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_action_proposals;
DROP TABLE IF EXISTS agent_messages;
DROP TABLE IF EXISTS trip_agents;
-- +goose StatementEnd
