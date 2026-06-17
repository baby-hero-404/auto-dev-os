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



CREATE TABLE IF NOT EXISTS credential_usage_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_id   UUID NOT NULL REFERENCES provider_credentials(id),

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
