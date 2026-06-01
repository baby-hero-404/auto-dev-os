-- 000008_episodic_memory.up.sql
-- Phase 6: 4-Tier Episodic Memory, Knowledge Graph, and Learning Suggestions

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. Episodic Memories — full 4-tier memory store
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE episodic_memories (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_id      UUID REFERENCES projects(id) ON DELETE SET NULL,
    task_id         UUID REFERENCES tasks(id) ON DELETE SET NULL,
    session_id      UUID,

    -- Tier classification: working | episodic | semantic | procedural
    tier            TEXT NOT NULL DEFAULT 'working'
                    CHECK (tier IN ('working', 'episodic', 'semantic', 'procedural')),

    -- Content
    content         TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    content_hash    TEXT NOT NULL DEFAULT '',

    -- Vector embedding (1536-dim for OpenAI ada-002 / equivalent)
    embedding       vector(1536),

    -- Metadata
    category        TEXT NOT NULL DEFAULT 'observation'
                    CHECK (category IN ('observation', 'decision', 'error', 'success', 'pattern', 'rule', 'tool_sequence')),
    tags            TEXT[] NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',

    -- Decay & reinforcement (Ebbinghaus curve)
    access_count    INT NOT NULL DEFAULT 0,
    decay_score     FLOAT NOT NULL DEFAULT 1.0,
    last_accessed   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- B-tree indexes for common filters
CREATE INDEX idx_episodic_memories_agent     ON episodic_memories(agent_id);
CREATE INDEX idx_episodic_memories_project   ON episodic_memories(project_id);
CREATE INDEX idx_episodic_memories_task      ON episodic_memories(task_id);
CREATE INDEX idx_episodic_memories_tier      ON episodic_memories(tier);
CREATE INDEX idx_episodic_memories_category  ON episodic_memories(category);
CREATE INDEX idx_episodic_memories_session   ON episodic_memories(session_id);
CREATE INDEX idx_episodic_memories_decay     ON episodic_memories(decay_score DESC);
CREATE INDEX idx_episodic_memories_hash      ON episodic_memories(content_hash);

-- BM25 full-text search using generated tsvector column
ALTER TABLE episodic_memories ADD COLUMN tsv tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(summary, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(content, '')), 'B')
    ) STORED;
CREATE INDEX idx_episodic_memories_tsv ON episodic_memories USING gin(tsv);

-- pgvector HNSW index for approximate nearest neighbor search
CREATE INDEX idx_episodic_memories_embedding ON episodic_memories
    USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. Knowledge Graph Edges — directed relations between memories
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE knowledge_edges (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id   UUID NOT NULL REFERENCES episodic_memories(id) ON DELETE CASCADE,
    target_id   UUID NOT NULL REFERENCES episodic_memories(id) ON DELETE CASCADE,
    relation    TEXT NOT NULL,  -- e.g. 'caused_by', 'solved_by', 'related_to', 'followed_by'
    weight      FLOAT NOT NULL DEFAULT 1.0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_id, target_id, relation)
);

CREATE INDEX idx_knowledge_edges_source   ON knowledge_edges(source_id);
CREATE INDEX idx_knowledge_edges_target   ON knowledge_edges(target_id);
CREATE INDEX idx_knowledge_edges_relation ON knowledge_edges(relation);

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. Learning Suggestions — HITL queue for agent self-improvement
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE learning_suggestions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_id      UUID REFERENCES projects(id) ON DELETE SET NULL,
    task_id         UUID REFERENCES tasks(id) ON DELETE SET NULL,

    -- Type: rule | prompt_patch | skill | pattern
    suggestion_type TEXT NOT NULL DEFAULT 'rule'
                    CHECK (suggestion_type IN ('rule', 'prompt_patch', 'skill', 'pattern')),

    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    content         TEXT NOT NULL DEFAULT '',
    confidence      FLOAT NOT NULL DEFAULT 0.5,

    -- Status: pending | approved | rejected | applied
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'applied')),

    reviewed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at     TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_learning_suggestions_agent  ON learning_suggestions(agent_id);
CREATE INDEX idx_learning_suggestions_status ON learning_suggestions(status);
CREATE INDEX idx_learning_suggestions_type   ON learning_suggestions(suggestion_type);
