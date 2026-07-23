CREATE TABLE IF NOT EXISTS attestation_keys (
    key_id TEXT PRIMARY KEY,
    public_key TEXT NOT NULL,
    private_key_encrypted TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS attestations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    job_id UUID,
    commit_hash TEXT NOT NULL,
    key_id TEXT NOT NULL REFERENCES attestation_keys(key_id),
    coded_by jsonb,
    reviewed_by jsonb,
    prompt_hash TEXT,
    policy_snapshot jsonb,
    envelope jsonb NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attestations_commit_hash ON attestations(commit_hash);
CREATE INDEX IF NOT EXISTS idx_attestations_task_id ON attestations(task_id);
