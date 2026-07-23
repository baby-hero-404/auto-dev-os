ALTER TABLE token_usage DROP COLUMN IF EXISTS cache_write_tokens;
ALTER TABLE token_usage DROP COLUMN IF EXISTS cache_read_tokens;
ALTER TABLE projects DROP COLUMN IF EXISTS smart_routing;
