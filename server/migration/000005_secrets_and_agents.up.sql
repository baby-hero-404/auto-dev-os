-- 000005_secrets_and_agents.up.sql
-- Phase 3a: secrets, org-scoped agents, project assignments, queue, checkpoints, logs.

ALTER TABLE agents
    ADD COLUMN org_id UUID,
    ADD COLUMN assignment_strategy TEXT NOT NULL DEFAULT 'manual';

UPDATE agents
SET org_id = projects.org_id
FROM projects
WHERE agents.project_id = projects.id;

ALTER TABLE agents
    ALTER COLUMN org_id SET NOT NULL,
    ALTER COLUMN project_id DROP NOT NULL,
    ADD CONSTRAINT agents_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

CREATE INDEX idx_agents_org_id ON agents(org_id);
CREATE INDEX idx_agents_assignment_strategy ON agents(assignment_strategy);

CREATE TABLE project_agents (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(project_id, agent_id)
);

INSERT INTO project_agents(project_id, agent_id)
SELECT project_id, id
FROM agents
WHERE project_id IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE TABLE secrets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    encrypted_value TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);
CREATE INDEX idx_secrets_project_id ON secrets(project_id);

CREATE TABLE workflow_jobs (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id   UUID REFERENCES agents(id) ON DELETE SET NULL,
    status     TEXT NOT NULL DEFAULT 'queued',
    step       TEXT NOT NULL DEFAULT 'analyze',
    attempts   INT NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_workflow_jobs_task_id ON workflow_jobs(task_id);
CREATE INDEX idx_workflow_jobs_status ON workflow_jobs(status);
CREATE INDEX idx_workflow_jobs_agent_id ON workflow_jobs(agent_id);

CREATE TABLE workflow_checkpoints (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    job_id     UUID REFERENCES workflow_jobs(id) ON DELETE CASCADE,
    step       TEXT NOT NULL,
    state      JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_workflow_checkpoints_task_id ON workflow_checkpoints(task_id);
CREATE INDEX idx_workflow_checkpoints_job_id ON workflow_checkpoints(job_id);

CREATE TABLE task_logs (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    job_id     UUID REFERENCES workflow_jobs(id) ON DELETE CASCADE,
    level      TEXT NOT NULL DEFAULT 'info',
    message    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX idx_task_logs_job_id ON task_logs(job_id);
CREATE INDEX idx_task_logs_created_at ON task_logs(created_at);
