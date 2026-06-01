-- 000009_workflow_artifacts.up.sql
-- Create table for workflow execution artifacts

CREATE TABLE workflow_artifacts (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id     UUID NOT NULL REFERENCES workflow_jobs(id) ON DELETE CASCADE,
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    step       TEXT NOT NULL,
    type       TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_artifacts_job_id ON workflow_artifacts(job_id);
CREATE INDEX idx_workflow_artifacts_task_id ON workflow_artifacts(task_id);
CREATE INDEX idx_workflow_artifacts_type ON workflow_artifacts(type);
CREATE INDEX idx_workflow_artifacts_created_at ON workflow_artifacts(created_at);
