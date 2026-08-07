CREATE TABLE task_action_requests (
    id uuid NOT NULL DEFAULT uuid_generate_v4() PRIMARY KEY,
    task_id uuid NOT NULL,
    request_id TEXT NOT NULL,
    action TEXT NOT NULL,
    response JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_task_action_requests_task_request ON task_action_requests (task_id, request_id);
