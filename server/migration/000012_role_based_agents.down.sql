-- Roll back role-based capability agents to the legacy provider/model/level schema.

UPDATE tasks SET agent_id = NULL WHERE agent_id IS NOT NULL;
UPDATE workflow_jobs SET agent_id = NULL WHERE agent_id IS NOT NULL;
UPDATE token_usage SET agent_id = NULL WHERE agent_id IS NOT NULL;
UPDATE virtual_keys SET agent_id = NULL WHERE agent_id IS NOT NULL;

DROP TABLE IF EXISTS role_templates CASCADE;
DROP TABLE IF EXISTS agent_skills CASCADE;
DROP TABLE IF EXISTS project_agents CASCADE;
DROP TABLE IF EXISTS agents CASCADE;

CREATE TABLE agents (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id          UUID REFERENCES projects(id),
    name                VARCHAR(100) NOT NULL,
    role                VARCHAR(50) DEFAULT 'backend',
    provider            VARCHAR(50) DEFAULT 'openai',
    model               VARCHAR(100) DEFAULT 'gpt-4o',
    level               VARCHAR(20) DEFAULT 'easy',
    status              VARCHAR(20) DEFAULT 'idle',
    assignment_strategy VARCHAR(50) DEFAULT 'manual',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_org_id ON agents(org_id);
CREATE INDEX idx_agents_project_id ON agents(project_id);
CREATE INDEX idx_agents_assignment_strategy ON agents(assignment_strategy);

CREATE TABLE project_agents (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(project_id, agent_id)
);

CREATE TABLE agent_skills (
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id   UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);
CREATE INDEX idx_agent_skills_skill_id ON agent_skills(skill_id);

ALTER TABLE tasks
    ADD CONSTRAINT tasks_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE workflow_jobs
    ADD CONSTRAINT workflow_jobs_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE token_usage
    ADD CONSTRAINT token_usage_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE virtual_keys
    ADD CONSTRAINT virtual_keys_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;
