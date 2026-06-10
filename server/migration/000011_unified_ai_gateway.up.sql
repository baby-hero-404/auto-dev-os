CREATE TABLE IF NOT EXISTS provider_credentials (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider        VARCHAR(50) NOT NULL,
    label           VARCHAR(100) NOT NULL DEFAULT 'default',
    encrypted_key   TEXT NOT NULL,
    base_url        TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    priority        INT NOT NULL DEFAULT 0,
    cooldown_until  TIMESTAMPTZ,
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, provider, label)
);

CREATE INDEX IF NOT EXISTS idx_provider_credentials_org_provider
    ON provider_credentials(org_id, provider, status);

CREATE TABLE IF NOT EXISTS virtual_keys (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id          UUID REFERENCES projects(id) ON DELETE CASCADE,
    agent_id            UUID REFERENCES agents(id) ON DELETE CASCADE,
    key_hash            VARCHAR(64) NOT NULL UNIQUE,
    key_prefix          VARCHAR(20) NOT NULL,
    name                VARCHAR(100) NOT NULL,
    budget_limit_usd    DECIMAL(10,4),
    budget_used_usd     DECIMAL(10,4) NOT NULL DEFAULT 0,
    rpm_limit           INT,
    tpm_limit           INT,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_virtual_keys_lookup ON virtual_keys(key_hash) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_virtual_keys_project ON virtual_keys(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_virtual_keys_agent ON virtual_keys(agent_id) WHERE agent_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS credential_usage_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_id   UUID NOT NULL REFERENCES provider_credentials(id),
    virtual_key_id  UUID REFERENCES virtual_keys(id),
    action          VARCHAR(30) NOT NULL,
    actor_type      VARCHAR(20) NOT NULL,
    actor_id        VARCHAR(100),
    detail          JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credential_usage_logs_cred
    ON credential_usage_logs(credential_id, created_at DESC);

CREATE TABLE IF NOT EXISTS model_routes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(50) NOT NULL,
    route_type      VARCHAR(20) NOT NULL,
    config          JSONB NOT NULL,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, name)
);
