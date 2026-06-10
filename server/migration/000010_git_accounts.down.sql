-- 000010_git_accounts.down.sql
-- Drop git_accounts table and reference in repositories

ALTER TABLE repositories DROP COLUMN git_account_id;
DROP TABLE git_accounts;
