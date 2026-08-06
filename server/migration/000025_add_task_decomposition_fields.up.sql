ALTER TABLE tasks ADD COLUMN sequence_index INT NULL;
ALTER TABLE tasks ADD COLUMN decomposition_mode TEXT NULL;
ALTER TABLE tasks ADD COLUMN complexity_score JSONB NULL;
ALTER TABLE tasks ADD COLUMN depends_on TEXT[] NULL;
ALTER TABLE tasks ADD COLUMN blocked_child_id uuid NULL;
