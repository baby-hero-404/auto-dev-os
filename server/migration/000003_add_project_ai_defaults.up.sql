ALTER TABLE projects
ADD COLUMN default_model_level text DEFAULT 'balanced' NOT NULL,
ADD COLUMN default_autonomy text DEFAULT 'supervised' NOT NULL,
ADD COLUMN auto_review_policy text DEFAULT 'complexity_based' NOT NULL,
ADD COLUMN max_retries integer DEFAULT 3 NOT NULL,
ADD COLUMN default_branch text DEFAULT 'main' NOT NULL;
