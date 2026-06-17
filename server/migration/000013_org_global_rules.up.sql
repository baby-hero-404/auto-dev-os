ALTER TABLE rules
    ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

UPDATE rules
SET org_id = projects.org_id
FROM projects
WHERE rules.project_id = projects.id
  AND rules.org_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_rules_org_id ON rules(org_id) WHERE org_id IS NOT NULL;
