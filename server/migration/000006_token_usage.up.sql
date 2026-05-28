-- 000006_token_usage.up.sql
-- Phase 4: LLM gateway token usage analytics and agent skill assignments.

CREATE TABLE token_usage (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id    UUID REFERENCES projects(id) ON DELETE SET NULL,
    agent_id      UUID REFERENCES agents(id) ON DELETE SET NULL,
    task_id       UUID REFERENCES tasks(id) ON DELETE SET NULL,
    provider      TEXT NOT NULL,
    model         TEXT NOT NULL,
    tier          TEXT NOT NULL DEFAULT 'balanced',
    prompt_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cost_usd      NUMERIC(12, 6) NOT NULL DEFAULT 0,
    latency_ms    INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'ok',
    error         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_token_usage_project_id ON token_usage(project_id);
CREATE INDEX idx_token_usage_agent_id ON token_usage(agent_id);
CREATE INDEX idx_token_usage_task_id ON token_usage(task_id);
CREATE INDEX idx_token_usage_created_at ON token_usage(created_at);

CREATE TABLE agent_skills (
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id   UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);
CREATE INDEX idx_agent_skills_skill_id ON agent_skills(skill_id);
