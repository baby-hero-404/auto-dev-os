-- 000010_git_accounts.up.sql
-- Create table for organization Git accounts and link to repositories

CREATE TABLE git_accounts (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,
    display_name TEXT NOT NULL,
    base_url     TEXT NOT NULL DEFAULT '',
    token        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, provider, display_name)
);

CREATE INDEX idx_git_accounts_org_id ON git_accounts(org_id);

ALTER TABLE repositories ADD COLUMN git_account_id UUID REFERENCES git_accounts(id) ON DELETE SET NULL;
