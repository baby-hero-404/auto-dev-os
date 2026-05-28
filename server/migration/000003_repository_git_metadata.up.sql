-- 000003_repository_git_metadata.up.sql
-- Phase 2: Repository clone/validation metadata

ALTER TABLE repositories
    ADD COLUMN clone_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN clone_status TEXT NOT NULL DEFAULT 'not_cloned',
    ADD COLUMN last_validated_at TIMESTAMPTZ;

CREATE INDEX idx_repositories_clone_status ON repositories(clone_status);
