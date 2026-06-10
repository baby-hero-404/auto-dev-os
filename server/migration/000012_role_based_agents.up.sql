-- Role-based capability agents.

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
    name                VARCHAR(100) NOT NULL,
    role                VARCHAR(100) NOT NULL,
    goal                TEXT NOT NULL,
    autonomy_level      VARCHAR(50) NOT NULL DEFAULT 'supervised',
    context_config      JSONB NOT NULL DEFAULT '{"max_input_tokens":128000}',
    model_route         VARCHAR(100) NOT NULL DEFAULT 'balanced',
    status              VARCHAR(20) NOT NULL DEFAULT 'idle',
    assignment_strategy VARCHAR(50) NOT NULL DEFAULT 'manual',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_org_id ON agents(org_id);
CREATE INDEX idx_agents_org_role ON agents(org_id, role);
CREATE INDEX idx_agents_assignment_strategy ON agents(assignment_strategy);
CREATE INDEX idx_agents_status ON agents(org_id, status) WHERE status IN ('idle', 'assigned');

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

CREATE TABLE role_templates (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role          VARCHAR(100) NOT NULL UNIQUE,
    default_goal  TEXT NOT NULL,
    default_tools JSONB NOT NULL DEFAULT '[]',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO role_templates (role, default_goal, default_tools) VALUES
    ('planner',          'Analyze tasks, create specs, and decompose into sub-tasks.',       '["analyze_codebase", "create_spec", "decompose_task"]'),
    ('backend',          'Develop backend code following clean architecture principles.',     '["read_file", "write_file", "run_tests", "git_commit"]'),
    ('frontend',         'Develop user interface components and user-facing workflows.',      '["read_file", "write_file", "run_tests", "git_commit"]'),
    ('reviewer',         'Review code changes and provide quality feedback.',                '["read_file", "analyze_diff", "add_review_comment"]'),
    ('qa',               'Test and ensure code quality through automated testing.',          '["run_tests", "analyze_logs", "read_file"]'),
    ('security-auditor', 'Scan for vulnerabilities and verify secret safety.',               '["scan_vulnerabilities", "read_file", "analyze_logs"]'),
    ('db-architect',     'Design schemas, create migrations, and optimize queries.',         '["read_file", "write_file", "create_migration", "run_tests"]');

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
