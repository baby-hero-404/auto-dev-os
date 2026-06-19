ALTER TABLE tasks ADD COLUMN pr_url text DEFAULT '';

UPDATE tasks SET pr_url = pr_urls[1] WHERE array_length(pr_urls, 1) > 0;

ALTER TABLE tasks DROP COLUMN pr_urls;
