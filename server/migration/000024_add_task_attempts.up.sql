CREATE TABLE task_attempts (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NULL,
    exit_status INT NULL,
    tokens_in INT NULL,
    tokens_out INT NULL,
    cost_usd NUMERIC NULL,
    sandbox_ref TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_task_attempts_task_id ON task_attempts(task_id);
