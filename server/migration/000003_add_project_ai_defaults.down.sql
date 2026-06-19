ALTER TABLE projects
DROP COLUMN IF EXISTS default_model_level,
DROP COLUMN IF EXISTS default_autonomy,
DROP COLUMN IF EXISTS auto_review_policy,
DROP COLUMN IF EXISTS max_retries,
DROP COLUMN IF EXISTS default_branch;
