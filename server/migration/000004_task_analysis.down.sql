-- 000004_task_analysis.down.sql

DROP INDEX IF EXISTS idx_tasks_spec_status;
DROP INDEX IF EXISTS idx_tasks_parent_task_id;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS spec_status,
    DROP COLUMN IF EXISTS analysis,
    DROP COLUMN IF EXISTS parent_task_id;
