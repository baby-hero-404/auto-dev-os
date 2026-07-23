CREATE TABLE IF NOT EXISTS learned_skills (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    trigger_keywords TEXT[] NOT NULL DEFAULT '{}',
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    source_task_id UUID,
    usage_count INT NOT NULL DEFAULT 0,
    success_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_learned_skills_project_status ON learned_skills(project_id, status);
