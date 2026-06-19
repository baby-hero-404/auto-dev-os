DROP INDEX IF EXISTS idx_tasks_repository_id;
ALTER TABLE tasks DROP COLUMN repository_id;
