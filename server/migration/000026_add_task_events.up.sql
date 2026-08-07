CREATE TABLE task_events (
    id uuid NOT NULL DEFAULT uuid_generate_v4() PRIMARY KEY,
    task_id uuid NOT NULL,
    sequence_number BIGINT NOT NULL,
    type TEXT NOT NULL,
    schema_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL DEFAULT '{}',
    artifact_id uuid NULL,
    size_bytes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_task_events_task_seq ON task_events (task_id, sequence_number);
CREATE INDEX idx_task_events_task_created ON task_events (task_id, created_at);
