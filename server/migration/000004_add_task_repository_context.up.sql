ALTER TABLE tasks ADD COLUMN repository_id uuid REFERENCES repositories(id) ON DELETE SET NULL;
CREATE INDEX idx_tasks_repository_id ON tasks USING btree (repository_id);
