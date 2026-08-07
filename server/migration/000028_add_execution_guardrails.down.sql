ALTER TABLE tasks DROP COLUMN IF EXISTS retry_count;
ALTER TABLE tasks DROP COLUMN IF EXISTS execution_started_at;
ALTER TABLE tasks DROP COLUMN IF EXISTS block_reason;

ALTER TABLE projects DROP COLUMN IF EXISTS max_task_retry_count;
ALTER TABLE projects DROP COLUMN IF EXISTS max_execution_minutes;
ALTER TABLE projects DROP COLUMN IF EXISTS max_event_count;
ALTER TABLE projects DROP COLUMN IF EXISTS cost_budget;
