-- 000004_task_analysis.up.sql
-- Phase 2: Task analysis, spec review, and sub-task support

ALTER TABLE tasks
    ADD COLUMN parent_task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
    ADD COLUMN analysis JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN spec_status TEXT NOT NULL DEFAULT 'none';

CREATE INDEX idx_tasks_parent_task_id ON tasks(parent_task_id);
CREATE INDEX idx_tasks_spec_status ON tasks(spec_status);
