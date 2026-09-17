-- Restores review_policy / autonomy_level as empty columns — the gate
-- settings chosen after this migration ran are lost and must be re-entered.

ALTER TABLE projects ADD COLUMN IF NOT EXISTS auto_review_policy text DEFAULT 'complexity_based' NOT NULL;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS autonomy_level character varying(50) DEFAULT 'supervised' NOT NULL;

ALTER TABLE projects DROP COLUMN IF EXISTS require_spec_review;
ALTER TABLE tasks DROP COLUMN IF EXISTS require_spec_review;
