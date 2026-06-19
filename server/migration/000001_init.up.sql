CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;
-- ========================================
-- Table: agent_skills
-- ========================================

CREATE TABLE agent_skills (
    agent_id uuid NOT NULL,
    skill_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY agent_skills
    ADD CONSTRAINT agent_skills_pkey PRIMARY KEY (agent_id, skill_id);

CREATE INDEX idx_agent_skills_skill_id ON agent_skills USING btree (skill_id);
-- ========================================
-- Table: agents
-- ========================================

CREATE TABLE agents (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    role character varying(100) NOT NULL,
    goal text NOT NULL,
    autonomy_level character varying(50) DEFAULT 'supervised'::character varying NOT NULL,
    context_config jsonb DEFAULT '{"max_input_tokens": 128000}'::jsonb NOT NULL,
    model_level_group character varying(100) DEFAULT 'balanced'::character varying NOT NULL,
    status character varying(20) DEFAULT 'idle'::character varying NOT NULL,
    assignment_strategy character varying(50) DEFAULT 'manual'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY agents
    ADD CONSTRAINT agents_pkey PRIMARY KEY (id);

CREATE INDEX idx_agents_assignment_strategy ON agents USING btree (assignment_strategy);

CREATE INDEX idx_agents_org_id ON agents USING btree (org_id);

CREATE INDEX idx_agents_org_role ON agents USING btree (org_id, role);

CREATE INDEX idx_agents_status ON agents USING btree (org_id, status) WHERE ((status)::text = ANY ((ARRAY['idle'::character varying, 'assigned'::character varying])::text[]));
-- ========================================
-- Table: api_keys
-- ========================================

CREATE TABLE api_keys (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    key_hash text NOT NULL,
    last_used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY api_keys
    ADD CONSTRAINT api_keys_key_hash_key UNIQUE (key_hash);

ALTER TABLE ONLY api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);

CREATE INDEX idx_api_keys_user_id ON api_keys USING btree (user_id);
-- ========================================
-- Table: audit_logs
-- ========================================

CREATE TABLE audit_logs (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid,
    user_id uuid,
    agent_id uuid,
    task_id uuid,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id text DEFAULT ''::text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    ip_address text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);

CREATE INDEX idx_audit_logs_action ON audit_logs USING btree (action);

CREATE INDEX idx_audit_logs_agent_id ON audit_logs USING btree (agent_id);

CREATE INDEX idx_audit_logs_created_at ON audit_logs USING btree (created_at DESC);

CREATE INDEX idx_audit_logs_entity_type ON audit_logs USING btree (entity_type);

CREATE INDEX idx_audit_logs_org_created ON audit_logs USING btree (org_id, created_at DESC);

CREATE INDEX idx_audit_logs_org_id ON audit_logs USING btree (org_id);

CREATE INDEX idx_audit_logs_task_id ON audit_logs USING btree (task_id);

CREATE INDEX idx_audit_logs_user_id ON audit_logs USING btree (user_id);
-- ========================================
-- Table: credential_usage_logs
-- ========================================

CREATE TABLE credential_usage_logs (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    credential_id uuid NOT NULL,
    action character varying(30) NOT NULL,
    actor_type character varying(20) NOT NULL,
    actor_id character varying(100),
    detail jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY credential_usage_logs
    ADD CONSTRAINT credential_usage_logs_pkey PRIMARY KEY (id);

CREATE INDEX idx_credential_usage_logs_cred ON credential_usage_logs USING btree (credential_id, created_at DESC);
-- ========================================
-- Table: episodic_memories
-- ========================================

CREATE TABLE episodic_memories (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    agent_id uuid NOT NULL,
    project_id uuid,
    task_id uuid,
    session_id uuid,
    tier text DEFAULT 'working'::text NOT NULL,
    content text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    content_hash text DEFAULT ''::text NOT NULL,
    embedding vector(1536),
    category text DEFAULT 'observation'::text NOT NULL,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    access_count integer DEFAULT 0 NOT NULL,
    decay_score double precision DEFAULT 1.0 NOT NULL,
    last_accessed timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tsv tsvector GENERATED ALWAYS AS ((setweight(to_tsvector('english'::regconfig, COALESCE(summary, ''::text)), 'A'::"char") || setweight(to_tsvector('english'::regconfig, COALESCE(content, ''::text)), 'B'::"char"))) STORED,
    CONSTRAINT episodic_memories_category_check CHECK ((category = ANY (ARRAY['observation'::text, 'decision'::text, 'error'::text, 'success'::text, 'pattern'::text, 'rule'::text, 'tool_sequence'::text]))),
    CONSTRAINT episodic_memories_tier_check CHECK ((tier = ANY (ARRAY['working'::text, 'episodic'::text, 'semantic'::text, 'procedural'::text])))
);

ALTER TABLE ONLY episodic_memories
    ADD CONSTRAINT episodic_memories_pkey PRIMARY KEY (id);

CREATE INDEX idx_episodic_memories_agent ON episodic_memories USING btree (agent_id);

CREATE INDEX idx_episodic_memories_category ON episodic_memories USING btree (category);

CREATE INDEX idx_episodic_memories_decay ON episodic_memories USING btree (decay_score DESC);

CREATE INDEX idx_episodic_memories_embedding ON episodic_memories USING hnsw (embedding vector_cosine_ops) WITH (m='16', ef_construction='64');

CREATE INDEX idx_episodic_memories_hash ON episodic_memories USING btree (content_hash);

CREATE INDEX idx_episodic_memories_project ON episodic_memories USING btree (project_id);

CREATE INDEX idx_episodic_memories_session ON episodic_memories USING btree (session_id);

CREATE INDEX idx_episodic_memories_task ON episodic_memories USING btree (task_id);

CREATE INDEX idx_episodic_memories_tier ON episodic_memories USING btree (tier);

CREATE INDEX idx_episodic_memories_tsv ON episodic_memories USING gin (tsv);
-- ========================================
-- Table: git_accounts
-- ========================================

CREATE TABLE git_accounts (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid NOT NULL,
    provider text NOT NULL,
    display_name text NOT NULL,
    base_url text DEFAULT ''::text NOT NULL,
    encrypted_token text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY git_accounts
    ADD CONSTRAINT git_accounts_org_id_provider_display_name_key UNIQUE (org_id, provider, display_name);

ALTER TABLE ONLY git_accounts
    ADD CONSTRAINT git_accounts_pkey PRIMARY KEY (id);

CREATE INDEX idx_git_accounts_org_id ON git_accounts USING btree (org_id);
-- ========================================
-- Table: knowledge_edges
-- ========================================

CREATE TABLE knowledge_edges (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    source_id uuid NOT NULL,
    target_id uuid NOT NULL,
    relation text NOT NULL,
    weight double precision DEFAULT 1.0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY knowledge_edges
    ADD CONSTRAINT knowledge_edges_pkey PRIMARY KEY (id);

ALTER TABLE ONLY knowledge_edges
    ADD CONSTRAINT knowledge_edges_source_id_target_id_relation_key UNIQUE (source_id, target_id, relation);

CREATE INDEX idx_knowledge_edges_relation ON knowledge_edges USING btree (relation);

CREATE INDEX idx_knowledge_edges_source ON knowledge_edges USING btree (source_id);

CREATE INDEX idx_knowledge_edges_target ON knowledge_edges USING btree (target_id);
-- ========================================
-- Table: learning_suggestions
-- ========================================

CREATE TABLE learning_suggestions (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    agent_id uuid NOT NULL,
    project_id uuid,
    task_id uuid,
    suggestion_type text DEFAULT 'rule'::text NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    confidence double precision DEFAULT 0.5 NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    reviewed_by uuid,
    reviewed_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT learning_suggestions_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text, 'applied'::text]))),
    CONSTRAINT learning_suggestions_suggestion_type_check CHECK ((suggestion_type = ANY (ARRAY['rule'::text, 'prompt_patch'::text, 'skill'::text, 'pattern'::text])))
);

ALTER TABLE ONLY learning_suggestions
    ADD CONSTRAINT learning_suggestions_pkey PRIMARY KEY (id);

CREATE INDEX idx_learning_suggestions_agent ON learning_suggestions USING btree (agent_id);

CREATE INDEX idx_learning_suggestions_status ON learning_suggestions USING btree (status);

CREATE INDEX idx_learning_suggestions_type ON learning_suggestions USING btree (suggestion_type);
-- ========================================
-- Table: memories
-- ========================================

CREATE TABLE memories (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    agent_id uuid NOT NULL,
    content text NOT NULL,
    embedding vector(1536),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY memories
    ADD CONSTRAINT memories_pkey PRIMARY KEY (id);

CREATE INDEX idx_memories_agent_id ON memories USING btree (agent_id);
-- ========================================
-- Table: organizations
-- ========================================

CREATE TABLE organizations (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);
-- ========================================
-- Table: project_agents
-- ========================================

CREATE TABLE project_agents (
    project_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY project_agents
    ADD CONSTRAINT project_agents_pkey PRIMARY KEY (project_id, agent_id);
-- ========================================
-- Table: projects
-- ========================================

CREATE TABLE projects (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_org_id_name_key UNIQUE (org_id, name);

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);

CREATE INDEX idx_projects_org_id ON projects USING btree (org_id);
-- ========================================
-- Table: provider_credentials
-- ========================================

CREATE TABLE provider_credentials (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid NOT NULL,
    provider character varying(50) NOT NULL,
    label character varying(100) DEFAULT 'default'::character varying NOT NULL,
    encrypted_key text NOT NULL,
    base_url text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    cooldown_until timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY provider_credentials
    ADD CONSTRAINT provider_credentials_org_id_provider_label_key UNIQUE (org_id, provider, label);

ALTER TABLE ONLY provider_credentials
    ADD CONSTRAINT provider_credentials_pkey PRIMARY KEY (id);

CREATE INDEX idx_provider_credentials_org_provider ON provider_credentials USING btree (org_id, provider, status);
-- ========================================
-- Table: provider_models
-- ========================================

CREATE TABLE provider_models (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    org_id uuid NOT NULL,
    provider character varying(50) NOT NULL,
    level_group character varying(50) NOT NULL,
    model_name character varying(100) NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY provider_models
    ADD CONSTRAINT provider_models_org_id_provider_level_group_model_name_key UNIQUE (org_id, provider, level_group, model_name);

ALTER TABLE ONLY provider_models
    ADD CONSTRAINT provider_models_pkey PRIMARY KEY (id);

CREATE INDEX idx_provider_models_org_level ON provider_models USING btree (org_id, level_group, is_active, priority);
-- ========================================
-- Table: repositories
-- ========================================

CREATE TABLE repositories (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    project_id uuid NOT NULL,
    url text NOT NULL,
    provider text DEFAULT 'github'::text NOT NULL,
    branch text DEFAULT 'main'::text NOT NULL,
    encrypted_token text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    clone_path text DEFAULT ''::text NOT NULL,
    clone_status text DEFAULT 'not_cloned'::text NOT NULL,
    last_validated_at timestamp with time zone,
    git_account_id uuid
);

ALTER TABLE ONLY repositories
    ADD CONSTRAINT repositories_pkey PRIMARY KEY (id);

CREATE INDEX idx_repositories_clone_status ON repositories USING btree (clone_status);

CREATE INDEX idx_repositories_project_id ON repositories USING btree (project_id);
-- ========================================
-- Table: role_templates
-- ========================================

CREATE TABLE role_templates (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    role character varying(100) NOT NULL,
    default_goal text NOT NULL,
    default_tools jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY role_templates
    ADD CONSTRAINT role_templates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY role_templates
    ADD CONSTRAINT role_templates_role_key UNIQUE (role);

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('planner', 'Analyze tasks, create specs, and decompose into sub-tasks.', '["analyze_codebase", "create_spec", "decompose_task"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('backend', 'Develop backend code following clean architecture principles.', '["read_file", "write_file", "run_tests", "git_commit"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('frontend', 'Develop user interface components and user-facing workflows.', '["read_file", "write_file", "run_tests", "git_commit"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('reviewer', 'Review code changes and provide quality feedback.', '["read_file", "analyze_diff", "add_review_comment"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('qa', 'Test and ensure code quality through automated testing.', '["run_tests", "analyze_logs", "read_file"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('security-auditor', 'Scan for vulnerabilities and verify secret safety.', '["scan_vulnerabilities", "read_file", "analyze_logs"]');

INSERT INTO role_templates (role, default_goal, default_tools) VALUES ('db-architect', 'Design schemas, create migrations, and optimize queries.', '["read_file", "write_file", "create_migration", "run_tests"]');
-- ========================================
-- Table: rules
-- ========================================

CREATE TABLE rules (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    project_id uuid,
    scope text DEFAULT 'project'::text NOT NULL,
    content text NOT NULL,
    enforcement text DEFAULT 'strict'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid
);

ALTER TABLE ONLY rules
    ADD CONSTRAINT rules_pkey PRIMARY KEY (id);

CREATE INDEX idx_rules_org_id ON rules USING btree (org_id) WHERE (org_id IS NOT NULL);

CREATE INDEX idx_rules_project_id ON rules USING btree (project_id);

CREATE INDEX idx_rules_scope ON rules USING btree (scope);
-- ========================================
-- Table: secrets
-- ========================================

CREATE TABLE secrets (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    project_id uuid NOT NULL,
    name text NOT NULL,
    encrypted_value text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY secrets
    ADD CONSTRAINT secrets_pkey PRIMARY KEY (id);

ALTER TABLE ONLY secrets
    ADD CONSTRAINT secrets_project_id_name_key UNIQUE (project_id, name);

CREATE INDEX idx_secrets_project_id ON secrets USING btree (project_id);
-- ========================================
-- Table: skills
-- ========================================

CREATE TABLE skills (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    schema jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY skills
    ADD CONSTRAINT skills_name_key UNIQUE (name);

ALTER TABLE ONLY skills
    ADD CONSTRAINT skills_pkey PRIMARY KEY (id);
-- ========================================
-- Table: task_logs
-- ========================================

CREATE TABLE task_logs (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    task_id uuid NOT NULL,
    job_id uuid,
    level text DEFAULT 'info'::text NOT NULL,
    message text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY task_logs
    ADD CONSTRAINT task_logs_pkey PRIMARY KEY (id);

CREATE INDEX idx_task_logs_created_at ON task_logs USING btree (created_at);

CREATE INDEX idx_task_logs_job_id ON task_logs USING btree (job_id);

CREATE INDEX idx_task_logs_task_id ON task_logs USING btree (task_id);
-- ========================================
-- Table: tasks
-- ========================================

CREATE TABLE tasks (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    project_id uuid NOT NULL,
    agent_id uuid,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'todo'::text NOT NULL,
    complexity text DEFAULT 'easy'::text NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    labels text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    parent_task_id uuid,
    analysis jsonb DEFAULT '{}'::jsonb NOT NULL,
    spec_status text DEFAULT 'none'::text NOT NULL,
    pr_url text DEFAULT ''::text NOT NULL
);

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (id);

CREATE INDEX idx_tasks_agent_id ON tasks USING btree (agent_id);

CREATE INDEX idx_tasks_parent_task_id ON tasks USING btree (parent_task_id);

CREATE INDEX idx_tasks_project_id ON tasks USING btree (project_id);

CREATE INDEX idx_tasks_spec_status ON tasks USING btree (spec_status);

CREATE INDEX idx_tasks_status ON tasks USING btree (status);
-- ========================================
-- Table: token_usage
-- ========================================

CREATE TABLE token_usage (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    project_id uuid,
    agent_id uuid,
    task_id uuid,
    provider text NOT NULL,
    model text NOT NULL,
    level_group text DEFAULT 'balanced'::text NOT NULL,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    output_tokens integer DEFAULT 0 NOT NULL,
    cost_usd numeric(12,6) DEFAULT 0 NOT NULL,
    latency_ms integer DEFAULT 0 NOT NULL,
    status text DEFAULT 'ok'::text NOT NULL,
    error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    org_id uuid,
    credential_id uuid
);

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_pkey PRIMARY KEY (id);

CREATE INDEX idx_token_usage_agent_id ON token_usage USING btree (agent_id);

CREATE INDEX idx_token_usage_created_at ON token_usage USING btree (created_at);

CREATE INDEX idx_token_usage_credential_id ON token_usage USING btree (credential_id);

CREATE INDEX idx_token_usage_org_id ON token_usage USING btree (org_id);

CREATE INDEX idx_token_usage_project_id ON token_usage USING btree (project_id);

CREATE INDEX idx_token_usage_task_id ON token_usage USING btree (task_id);
-- ========================================
-- Table: users
-- ========================================

CREATE TABLE users (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    org_id uuid NOT NULL,
    role text DEFAULT 'admin'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE INDEX idx_users_org_id ON users USING btree (org_id);
-- ========================================
-- Table: workflow_artifacts
-- ========================================

CREATE TABLE workflow_artifacts (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    job_id uuid NOT NULL,
    task_id uuid NOT NULL,
    step text NOT NULL,
    type text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY workflow_artifacts
    ADD CONSTRAINT workflow_artifacts_pkey PRIMARY KEY (id);

CREATE INDEX idx_workflow_artifacts_created_at ON workflow_artifacts USING btree (created_at);

CREATE INDEX idx_workflow_artifacts_job_id ON workflow_artifacts USING btree (job_id);

CREATE INDEX idx_workflow_artifacts_task_id ON workflow_artifacts USING btree (task_id);

CREATE INDEX idx_workflow_artifacts_type ON workflow_artifacts USING btree (type);
-- ========================================
-- Table: workflow_checkpoints
-- ========================================

CREATE TABLE workflow_checkpoints (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    task_id uuid NOT NULL,
    job_id uuid,
    step text NOT NULL,
    state jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY workflow_checkpoints
    ADD CONSTRAINT workflow_checkpoints_pkey PRIMARY KEY (id);

CREATE INDEX idx_workflow_checkpoints_job_id ON workflow_checkpoints USING btree (job_id);

CREATE INDEX idx_workflow_checkpoints_task_id ON workflow_checkpoints USING btree (task_id);
-- ========================================
-- Table: workflow_jobs
-- ========================================

CREATE TABLE workflow_jobs (
    id uuid DEFAULT uuid_generate_v4() NOT NULL,
    task_id uuid NOT NULL,
    agent_id uuid,
    status text DEFAULT 'queued'::text NOT NULL,
    step text DEFAULT 'analyze'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY workflow_jobs
    ADD CONSTRAINT workflow_jobs_pkey PRIMARY KEY (id);

CREATE INDEX idx_workflow_jobs_agent_id ON workflow_jobs USING btree (agent_id);

CREATE INDEX idx_workflow_jobs_status ON workflow_jobs USING btree (status);

CREATE INDEX idx_workflow_jobs_task_id ON workflow_jobs USING btree (task_id);
-- ========================================
-- Foreign Key Constraints
-- ========================================

ALTER TABLE ONLY agent_skills
    ADD CONSTRAINT agent_skills_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE ONLY agent_skills
    ADD CONSTRAINT agent_skills_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE;

ALTER TABLE ONLY agents
    ADD CONSTRAINT agents_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY api_keys
    ADD CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE ONLY credential_usage_logs
    ADD CONSTRAINT credential_usage_logs_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES provider_credentials(id);

ALTER TABLE ONLY episodic_memories
    ADD CONSTRAINT episodic_memories_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE ONLY episodic_memories
    ADD CONSTRAINT episodic_memories_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;

ALTER TABLE ONLY git_accounts
    ADD CONSTRAINT git_accounts_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY knowledge_edges
    ADD CONSTRAINT knowledge_edges_source_id_fkey FOREIGN KEY (source_id) REFERENCES episodic_memories(id) ON DELETE CASCADE;

ALTER TABLE ONLY knowledge_edges
    ADD CONSTRAINT knowledge_edges_target_id_fkey FOREIGN KEY (target_id) REFERENCES episodic_memories(id) ON DELETE CASCADE;

ALTER TABLE ONLY learning_suggestions
    ADD CONSTRAINT learning_suggestions_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE ONLY learning_suggestions
    ADD CONSTRAINT learning_suggestions_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE ONLY learning_suggestions
    ADD CONSTRAINT learning_suggestions_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;

ALTER TABLE ONLY project_agents
    ADD CONSTRAINT project_agents_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE ONLY project_agents
    ADD CONSTRAINT project_agents_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY provider_credentials
    ADD CONSTRAINT provider_credentials_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY provider_models
    ADD CONSTRAINT provider_models_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY repositories
    ADD CONSTRAINT repositories_git_account_id_fkey FOREIGN KEY (git_account_id) REFERENCES git_accounts(id) ON DELETE SET NULL;

ALTER TABLE ONLY repositories
    ADD CONSTRAINT repositories_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE ONLY rules
    ADD CONSTRAINT rules_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY rules
    ADD CONSTRAINT rules_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE ONLY secrets
    ADD CONSTRAINT secrets_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE ONLY task_logs
    ADD CONSTRAINT task_logs_job_id_fkey FOREIGN KEY (job_id) REFERENCES workflow_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY task_logs
    ADD CONSTRAINT task_logs_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_parent_task_id_fkey FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE CASCADE;

ALTER TABLE ONLY tasks
    ADD CONSTRAINT tasks_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES provider_credentials(id) ON DELETE SET NULL;

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE ONLY token_usage
    ADD CONSTRAINT token_usage_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;

ALTER TABLE ONLY users
    ADD CONSTRAINT users_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY workflow_artifacts
    ADD CONSTRAINT workflow_artifacts_job_id_fkey FOREIGN KEY (job_id) REFERENCES workflow_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY workflow_artifacts
    ADD CONSTRAINT workflow_artifacts_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;

ALTER TABLE ONLY workflow_checkpoints
    ADD CONSTRAINT workflow_checkpoints_job_id_fkey FOREIGN KEY (job_id) REFERENCES workflow_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY workflow_checkpoints
    ADD CONSTRAINT workflow_checkpoints_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;

ALTER TABLE ONLY workflow_jobs
    ADD CONSTRAINT workflow_jobs_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE ONLY workflow_jobs
    ADD CONSTRAINT workflow_jobs_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;
