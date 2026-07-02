INSERT INTO role_templates (role, default_goal, default_tools)
VALUES ('documentation-writer', 'Write and update high-quality project documentation, readme files, and user guides.', '["read_file", "write_file", "git_commit"]')
ON CONFLICT (role) DO NOTHING;
