ALTER TABLE projects ADD COLUMN IF NOT EXISTS max_review_fix_cycles integer NOT NULL DEFAULT 3;
