-- Replaces the review-policy/autonomy matrix with a single boolean gate:
-- require_spec_review. See server/internal/policy/review_policy.go.

ALTER TABLE projects ADD COLUMN require_spec_review boolean NOT NULL DEFAULT false;
ALTER TABLE tasks ADD COLUMN require_spec_review boolean;

UPDATE projects SET require_spec_review = true WHERE auto_review_policy = 'always_review';

ALTER TABLE projects DROP COLUMN auto_review_policy;
ALTER TABLE agents DROP COLUMN autonomy_level;
