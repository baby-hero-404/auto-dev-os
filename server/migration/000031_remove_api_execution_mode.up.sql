-- Remove API-native execution mode entirely (clean break, no fallback).
-- CLI execution via project/org execution_providers rows is now the only
-- supported path.

-- Drop any non-CLI provider credentials outright — API-native credentials
-- have no remaining reader.
DELETE FROM provider_credentials WHERE provider NOT LIKE 'cli:%';

-- Strip any "type":"api" rows out of the execution_providers jsonb arrays so
-- stale API-typed config can never be read back after the columns/consumers
-- that resolved them are gone.
UPDATE projects
SET execution_providers = COALESCE(
	(SELECT jsonb_agg(elem) FROM jsonb_array_elements(execution_providers) elem WHERE elem->>'type' <> 'api'),
	'[]'::jsonb
)
WHERE execution_providers @> '[{"type":"api"}]'::jsonb;

UPDATE organizations
SET default_execution_providers = COALESCE(
	(SELECT jsonb_agg(elem) FROM jsonb_array_elements(default_execution_providers) elem WHERE elem->>'type' <> 'api'),
	'[]'::jsonb
)
WHERE default_execution_providers @> '[{"type":"api"}]'::jsonb;

-- Drop the legacy single-engine columns superseded by execution_providers.
ALTER TABLE projects DROP COLUMN IF EXISTS execution_engine;
ALTER TABLE projects DROP COLUMN IF EXISTS cli_engine_config;
ALTER TABLE tasks DROP COLUMN IF EXISTS execution_engine;

-- Drop API-native-only accounting tables.
DROP TABLE IF EXISTS token_usage;
DROP TABLE IF EXISTS provider_models;

-- Drop embeddings / vector memory search entirely.
DROP INDEX IF EXISTS idx_episodic_memories_embedding;
ALTER TABLE episodic_memories DROP COLUMN IF EXISTS embedding;
ALTER TABLE memories DROP COLUMN IF EXISTS embedding;
