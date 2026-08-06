ALTER TABLE tasks DROP COLUMN IF EXISTS blocked_child_id;
ALTER TABLE tasks DROP COLUMN IF EXISTS depends_on;
ALTER TABLE tasks DROP COLUMN IF EXISTS complexity_score;
ALTER TABLE tasks DROP COLUMN IF EXISTS decomposition_mode;
ALTER TABLE tasks DROP COLUMN IF EXISTS sequence_index;
