ALTER TABLE organizations ADD COLUMN default_execution_providers jsonb NOT NULL DEFAULT '[]';
