ALTER TABLE tasks ADD COLUMN pr_metadata JSONB DEFAULT '[]'::jsonb;
