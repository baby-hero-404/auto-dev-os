-- Chat conversations with a headless AI CLI (aicli conversation.Store).
--
-- chat_events.seq is consecutive per conversation (no gaps): appends lock the conversation row.
-- chat_turns.(conversation_id, client_message_id) is the idempotency key of a message.

CREATE TABLE IF NOT EXISTS chat_conversations (
    id             text PRIMARY KEY,
    owner          text NOT NULL,
    profile        text NOT NULL,
    title          text NOT NULL DEFAULT '',
    resumable      boolean NOT NULL DEFAULT true,
    session_id     text NOT NULL DEFAULT '',
    active_turn_id text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chat_conversations_owner ON chat_conversations(owner, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS chat_turns (
    id                    text PRIMARY KEY,
    conversation_id       text NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    client_message_id     text NOT NULL,
    status                text NOT NULL CHECK (status IN ('running','completed','failed','cancelled')),
    failure               text NOT NULL DEFAULT '',
    retry_after_ms        bigint NOT NULL DEFAULT 0,
    credentials_not_saved boolean NOT NULL DEFAULT false,
    cancel_requested      boolean NOT NULL DEFAULT false,
    lease_until           timestamptz NOT NULL,
    started_at            timestamptz NOT NULL,
    finished_at           timestamptz,
    UNIQUE (conversation_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_turns_running ON chat_turns(lease_until) WHERE status = 'running';

CREATE TABLE IF NOT EXISTS chat_events (
    conversation_id text NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    seq             bigint NOT NULL,
    turn_id         text NOT NULL,
    at              timestamptz NOT NULL,
    kind            text NOT NULL,
    body            text NOT NULL DEFAULT '',
    tool_name       text,
    tool_summary    text,
    tool_is_error   boolean,
    meta            jsonb,
    PRIMARY KEY (conversation_id, seq)
);
