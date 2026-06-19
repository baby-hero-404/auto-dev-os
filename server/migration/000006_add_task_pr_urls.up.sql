ALTER TABLE tasks ADD COLUMN pr_urls text[] DEFAULT '{}';

-- Migration to move existing PR URLs if any
UPDATE tasks SET pr_urls = array_append(pr_urls, pr_url) WHERE pr_url != '' AND pr_url IS NOT NULL;

-- Note: We can drop pr_url if we want, but let's just leave it or drop it.
ALTER TABLE tasks DROP COLUMN pr_url;
