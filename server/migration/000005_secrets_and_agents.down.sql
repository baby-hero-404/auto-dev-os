-- 000005_secrets_and_agents.down.sql

DROP TABLE IF EXISTS task_logs;
DROP TABLE IF EXISTS workflow_checkpoints;
DROP TABLE IF EXISTS workflow_jobs;
DROP TABLE IF EXISTS secrets;
DROP TABLE IF EXISTS project_agents;

DROP INDEX IF EXISTS idx_agents_assignment_strategy;
DROP INDEX IF EXISTS idx_agents_org_id;

ALTER TABLE agents
    DROP CONSTRAINT IF EXISTS agents_org_id_fkey,
    DROP COLUMN IF EXISTS assignment_strategy,
    DROP COLUMN IF EXISTS org_id;

DELETE FROM agents WHERE project_id IS NULL;

ALTER TABLE agents
    ALTER COLUMN project_id SET NOT NULL;
