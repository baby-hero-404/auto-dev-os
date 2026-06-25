ALTER TABLE repositories
    ADD COLUMN IF NOT EXISTS display_name text DEFAULT ''::text NOT NULL;

UPDATE repositories
SET display_name = COALESCE(NULLIF(TRIM(display_name), ''), REGEXP_REPLACE(url, '^https?://', ''))
WHERE display_name IS NULL OR TRIM(display_name) = '';
