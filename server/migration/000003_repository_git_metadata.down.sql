-- 000003_repository_git_metadata.down.sql

DROP INDEX IF EXISTS idx_repositories_clone_status;

ALTER TABLE repositories
    DROP COLUMN IF EXISTS last_validated_at,
    DROP COLUMN IF EXISTS clone_status,
    DROP COLUMN IF EXISTS clone_path;
