-- Restores schema shape only — the deleted API-native rows, engine config,
-- accounting history, and embeddings are gone for good (clean break).

ALTER TABLE projects ADD COLUMN IF NOT EXISTS execution_engine text DEFAULT 'api_native'::text NOT NULL;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS cli_engine_config jsonb DEFAULT '{}'::jsonb NOT NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS execution_engine text;

CREATE TABLE IF NOT EXISTS provider_models (
	id uuid DEFAULT uuid_generate_v4() NOT NULL,
	org_id uuid NOT NULL,
	provider character varying(50) NOT NULL,
	level_group character varying(50) NOT NULL,
	model_name character varying(100) NOT NULL,
	priority integer DEFAULT 0 NOT NULL,
	is_active boolean DEFAULT true NOT NULL,
	created_at timestamp with time zone DEFAULT now() NOT NULL,
	updated_at timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT provider_models_pkey PRIMARY KEY (id),
	CONSTRAINT provider_models_org_id_provider_level_group_model_name_key UNIQUE (org_id, provider, level_group, model_name),
	CONSTRAINT provider_models_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_provider_models_org_level ON provider_models USING btree (org_id, level_group, is_active, priority);

CREATE TABLE IF NOT EXISTS token_usage (
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
	credential_id uuid,
	CONSTRAINT token_usage_pkey PRIMARY KEY (id),
	CONSTRAINT token_usage_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
	CONSTRAINT token_usage_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES provider_credentials(id) ON DELETE SET NULL,
	CONSTRAINT token_usage_org_id_fkey FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
	CONSTRAINT token_usage_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL,
	CONSTRAINT token_usage_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_token_usage_agent_id ON token_usage USING btree (agent_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_created_at ON token_usage USING btree (created_at);
CREATE INDEX IF NOT EXISTS idx_token_usage_credential_id ON token_usage USING btree (credential_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_org_id ON token_usage USING btree (org_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_project_id ON token_usage USING btree (project_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_task_id ON token_usage USING btree (task_id);

ALTER TABLE episodic_memories ADD COLUMN IF NOT EXISTS embedding vector(1536);
CREATE INDEX IF NOT EXISTS idx_episodic_memories_embedding ON episodic_memories USING hnsw (embedding vector_cosine_ops) WITH (m='16', ef_construction='64');
ALTER TABLE memories ADD COLUMN IF NOT EXISTS embedding vector(1536);
