ALTER TABLE projects ADD COLUMN IF NOT EXISTS review_harness_policy VARCHAR(32) NOT NULL DEFAULT 'different_model';
