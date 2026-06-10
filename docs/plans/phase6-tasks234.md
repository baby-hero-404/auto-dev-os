# Phase 6 Tasks 2, 3, 4 — Implementation Plan

> **Status:** 📋 Ready for Review
> **Depends on:** Phase 5 (100% complete)
> **Execution Order:** Task 2 → Task 3 → Task 4 (strictly sequential)

---

## Overview

| Task | Scope | Approach |
|------|-------|----------|
| **Task 2** | 4-Tier Episodic Memory (pgvector + BM25) | Full 4-tier model from day 1 |
| **Task 3** | Self-improving Agent Loop | Active with HITL — suggestions for human approval |
| **Task 4** | Web UI Memory Dashboard | Read-only viewer: browse memories, view learning history |

---

## Task 2: 4-Tier Episodic Memory & Knowledge Graph

### Sub-task 2.1 — Database Migration `000008`

**File:** `server/migration/000008_episodic_memory.up.sql` / `.down.sql`

Tables created:
- `episodic_memories` — Full 4-tier memory store with vector embeddings, BM25 tsvector, decay scoring
- `knowledge_edges` — Directed graph edges between memories (caused_by, solved_by, related_to, followed_by)
- `learning_suggestions` — HITL suggestion queue for Task 3

Key indexes:
- HNSW index on `embedding` column for approximate nearest neighbor via pgvector
- GIN index on generated `tsv` tsvector column for BM25 full-text search
- B-tree indexes on agent_id, project_id, tier, category, session_id, decay_score

---

### Sub-task 2.2 — Domain Models

**File:** `server/pkg/models/phase6.go`

| Model | Purpose |
|-------|---------|
| `EpisodicMemory` | GORM model for `episodic_memories` table |
| `KnowledgeEdge` | GORM model for `knowledge_edges` table |
| `LearningSuggestion` | GORM model for `learning_suggestions` table |
| `MemoryTier` constants | `working`, `episodic`, `semantic`, `procedural` |
| `MemoryCategory` constants | `observation`, `decision`, `error`, `success`, `pattern`, `rule`, `tool_sequence` |
| `SuggestionType` constants | `rule`, `prompt_patch`, `skill`, `pattern` |
| `SuggestionStatus` constants | `pending`, `approved`, `rejected`, `applied` |
| `CreateMemoryInput` | Input struct for recording a new memory |
| `MemorySearchInput` | Input struct for triple-stream search |
| `MemorySearchResult` | Result struct with merged RRF scores |
| `CreateSuggestionInput` | Input struct for proposing a learning suggestion |
| `UpdateSuggestionInput` | Input for approving/rejecting suggestions |

---

### Sub-task 2.3 — Repository Layer

**File:** `server/internal/repository/memory.go`

| Method | Description |
|--------|-------------|
| `Create(ctx, *EpisodicMemory) error` | Insert with content_hash dedup check |
| `GetByID(ctx, id) (*EpisodicMemory, error)` | Single fetch, bump access_count and last_accessed |
| `ListByAgent(ctx, agentID, tier, limit, offset)` | Paginated list with tier filter |
| `ListBySession(ctx, sessionID)` | All memories from a session |
| `SearchBM25(ctx, query, agentID, limit) []MemorySearchResult` | Full-text search via ts_rank |
| `SearchVector(ctx, embedding, agentID, limit) []MemorySearchResult` | pgvector cosine similarity |
| `SearchGraph(ctx, memoryID, depth) []MemorySearchResult` | Traverse knowledge_edges |
| `UpdateDecay(ctx) error` | Batch: decay_score *= 0.95 for memories not accessed in 7 days |
| `Delete(ctx, id) error` | Hard delete (for working memory cleanup) |
| `PruneWorkingMemory(ctx, agentID, keepCount) error` | Remove oldest working tier memories |

**File:** `server/internal/repository/knowledge_edge.go`

| Method | Description |
|--------|-------------|
| `Create(ctx, *KnowledgeEdge) error` | Insert edge with dedup |
| `ListBySource(ctx, sourceID) []KnowledgeEdge` | Outbound edges |
| `ListByTarget(ctx, targetID) []KnowledgeEdge` | Inbound edges |
| `Delete(ctx, id) error` | Remove edge |

**File:** `server/internal/repository/learning_suggestion.go`

| Method | Description |
|--------|-------------|
| `Create(ctx, *LearningSuggestion) error` | Insert new suggestion |
| `GetByID(ctx, id) (*LearningSuggestion, error)` | Single fetch |
| `List(ctx, agentID, status, limit) []LearningSuggestion` | Filtered list |
| `Update(ctx, id, UpdateSuggestionInput) (*LearningSuggestion, error)` | Approve/reject |

---

### Sub-task 2.4 — Service Layer (4-Tier Memory Store)

**File:** `server/internal/service/memory.go`

| Method | Description |
|--------|-------------|
| `RecordObservation(ctx, input)` | Hash-dedup, store as `working` tier |
| `PromoteToEpisodic(ctx, memoryID)` | Compress content to summary, move to `episodic` tier |
| `PromoteToSemantic(ctx, memoryID)` | Generalize into reusable fact, set `semantic` tier |
| `PromoteToProceduralIfPattern(ctx, agentID)` | Detect repeated tool sequences → `procedural` tier |
| `Search(ctx, MemorySearchInput) []MemorySearchResult` | Triple-stream: BM25 + Vector + Graph → RRF merge |
| `ApplyDecay(ctx) error` | Run Ebbinghaus decay batch |
| `CleanupSession(ctx, sessionID)` | Prune working memories, compress to episodic |

**RRF (Reciprocal Rank Fusion)** logic:
```
score(doc) = Σ 1/(k + rank_i(doc))   for each stream i
k = 60 (standard constant)
```

---

### Sub-task 2.5 — Lifecycle Hooks (Orchestrator Integration)

**File:** `server/internal/orchestrator/memory_hooks.go`

| Hook | Trigger Point | Behavior |
|------|---------------|----------|
| `SessionStart` | `orchestrator.run()` after agent assignment | Load relevant memories via triple-stream, inject into prompt context |
| `PostToolUse` | After each workflow step completes | Record observation (SHA-256 dedup), strip secrets via regex filter |
| `SessionEnd` | End of `orchestrator.run()` (success or failure) | Compile session summary → episodic, extract patterns → promote, link knowledge edges |

**Integration in `orchestrator.go`:**
- Add `memoryStore *service.MemoryService` field to `Orchestrator`.
- Call hooks at appropriate points in the `run()` method.

---

### Sub-task 2.6 — API Endpoints

**File:** `server/internal/handler/memory.go`

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/v1/agents/{agentID}/memories` | GET | List memories with tier/category filter + pagination |
| `GET /api/v1/agents/{agentID}/memories/search` | GET | Triple-stream search (query param: `q`) |
| `GET /api/v1/memories/{memoryID}` | GET | Single memory detail + linked edges |
| `DELETE /api/v1/memories/{memoryID}` | DELETE | Admin-only delete |

---

## Task 3: Self-improving Agent Loop

### Sub-task 3.1 — Post-Task Evaluation Engine

**File:** `server/internal/orchestrator/learning.go`

| Component | Description |
|-----------|-------------|
| `EvaluateOutcome(ctx, task, job)` | Classify outcome: success / failure / retried. Record as `decision` or `error` category memory |
| `DetectPatterns(ctx, agentID)` | Scan recent procedural memories for recurring tool sequences → propose skill |
| `SuggestRuleFromErrors(ctx, agentID)` | Analyze repeated coding mistakes → generate rule suggestion |
| `SuggestPromptPatch(ctx, task, retries)` | When retries > 2, analyze failure logs → suggest system prompt tweaks |
| `ComputeConfidence(ctx, agentID, taskID)` | Historical success rate × complexity weight → 0.0–1.0 score |

All suggestions flow through `LearningSuggestion` with status `pending` → human approves/rejects.

### Sub-task 3.2 — Orchestrator Integration

**Modify:** `server/internal/orchestrator/orchestrator.go`

- After `run()` completes (success or failure), call `EvaluateOutcome()`.
- After evaluation, call `DetectPatterns()` and `SuggestRuleFromErrors()`.
- On task failure with retries > 2, call `SuggestPromptPatch()`.
- Before execution, call `ComputeConfidence()` and store in workflow checkpoint.

### Sub-task 3.3 — HITL Suggestion Review API

**File:** `server/internal/handler/learning.go`

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/v1/agents/{agentID}/suggestions` | GET | List pending suggestions |
| `GET /api/v1/suggestions/{suggestionID}` | GET | Single suggestion detail |
| `POST /api/v1/suggestions/{suggestionID}/approve` | POST | Approve → apply (creates rule / patches prompt) |
| `POST /api/v1/suggestions/{suggestionID}/reject` | POST | Reject with feedback |

**File:** `server/internal/service/learning.go`

| Method | Description |
|--------|-------------|
| `ListSuggestions(ctx, agentID, status)` | Paginated list |
| `ApproveSuggestion(ctx, id, userID)` | Mark approved, auto-apply if type = `rule` |
| `RejectSuggestion(ctx, id, userID)` | Mark rejected, store feedback in metadata |
| `ApplySuggestion(ctx, suggestion)` | Execute: insert rule / update prompt template / register skill |

---

## Task 4: Web UI Memory & Learning Dashboard

### Sub-task 4.1 — Frontend Types & API Client

**File:** `web/src/lib/types.ts` (append Phase 6 types)

```typescript
// ─── Phase 6: Episodic Memory & Learning ─────────────────────────────

export type EpisodicMemory = {
  id: string;
  agent_id: string;
  project_id?: string;
  task_id?: string;
  session_id?: string;
  tier: 'working' | 'episodic' | 'semantic' | 'procedural';
  content: string;
  summary: string;
  category: string;
  tags: string[];
  metadata: Record<string, unknown>;
  access_count: number;
  decay_score: number;
  last_accessed: string;
  created_at: string;
};

export type KnowledgeEdge = {
  id: string;
  source_id: string;
  target_id: string;
  relation: string;
  weight: number;
  created_at: string;
};

export type LearningSuggestion = {
  id: string;
  agent_id: string;
  project_id?: string;
  task_id?: string;
  suggestion_type: 'rule' | 'prompt_patch' | 'skill' | 'pattern';
  title: string;
  description: string;
  content: string;
  confidence: number;
  status: 'pending' | 'approved' | 'rejected' | 'applied';
  reviewed_by?: string;
  reviewed_at?: string;
  metadata: Record<string, unknown>;
  created_at: string;
};
```

**File:** `web/src/lib/api.ts` (append memory & learning API functions)

### Sub-task 4.2 — Memory Browser Page

**File:** `web/src/app/knowledge/page.tsx` (enhance existing)

| UI Element | Description |
|------------|-------------|
| **Tier Filter Tabs** | Working / Episodic / Semantic / Procedural / All |
| **Search Bar** | Queries the triple-stream search endpoint |
| **Memory List** | Cards showing: summary, category badge, tier pill, decay bar, timestamp |
| **Memory Detail Panel** | Expand to see full content, linked edges, tags |
| **Agent Selector** | Dropdown to filter by agent |

### Sub-task 4.3 — Learning Suggestions Page

**File:** `web/src/app/knowledge/suggestions/page.tsx`

| UI Element | Description |
|------------|-------------|
| **Pending Suggestions List** | Cards with: title, type badge, confidence %, agent name |
| **Suggestion Detail** | Full description, proposed content (rule text / prompt diff) |
| **Approve / Reject Buttons** | Call POST endpoints, show toast confirmation |
| **History Tab** | Show approved + rejected suggestions with reviewer info |

---

## File Impact Summary

### New Files (15 files)

| # | Path | Layer |
|---|------|-------|
| 1 | `server/migration/000008_episodic_memory.up.sql` | Migration |
| 2 | `server/migration/000008_episodic_memory.down.sql` | Migration |
| 3 | `server/pkg/models/phase6.go` | Models |
| 4 | `server/internal/repository/memory.go` | Repository |
| 5 | `server/internal/repository/knowledge_edge.go` | Repository |
| 6 | `server/internal/repository/learning_suggestion.go` | Repository |
| 7 | `server/internal/service/memory.go` | Service |
| 8 | `server/internal/service/learning.go` | Service |
| 9 | `server/internal/orchestrator/memory_hooks.go` | Orchestrator |
| 10 | `server/internal/orchestrator/learning.go` | Orchestrator |
| 11 | `server/internal/handler/memory.go` | Handler |
| 12 | `server/internal/handler/learning.go` | Handler |
| 13 | `web/src/app/knowledge/suggestions/page.tsx` | Frontend |
| 14 | `server/internal/service/memory_test.go` | Test |
| 15 | `server/internal/service/learning_test.go` | Test |

### Modified Files (5 files)

| # | Path | Change |
|---|------|--------|
| 1 | `server/internal/handler/router.go` | Add memory + suggestion routes |
| 2 | `server/cmd/api/main.go` | Wire new repos, services, handler deps |
| 3 | `server/internal/orchestrator/orchestrator.go` | Add memory hooks + learning calls |
| 4 | `web/src/lib/types.ts` | Add Phase 6 types |
| 5 | `web/src/lib/api.ts` | Add memory + learning API functions |

### Optional enhancement to existing file

| # | Path | Change |
|---|------|--------|
| 1 | `web/src/app/knowledge/page.tsx` | Enhance with memory browser UI |

---

## Execution Order (15 sub-tasks)

```
TASK 2: Episodic Memory
  2.1  Migration 000008 (up + down)
  2.2  Domain models (phase6.go)
  2.3  Repository layer (memory, edge, suggestion repos)
  2.4  Service layer (memory service + RRF search)
  2.5  Lifecycle hooks (orchestrator integration)
  2.6  API handler + router wiring + main.go DI

TASK 3: Self-improving Loop
  3.1  Learning engine (evaluation, patterns, suggestions)
  3.2  Orchestrator integration (post-run hooks)
  3.3  Learning handler + suggestion review API

TASK 4: Web UI
  4.1  Frontend types + API client
  4.2  Memory browser page
  4.3  Learning suggestions page
```

---

## Testing Strategy

| Layer | Type | Scope |
|-------|------|-------|
| `memory_test.go` | Unit | RRF fusion scoring, hash dedup, decay calculation |
| `learning_test.go` | Unit | Pattern detection, confidence scoring, suggestion lifecycle |
| Repository tests | Integration | BM25 search accuracy, vector search, edge traversal |
| Handler tests | HTTP | Endpoint validation, auth, error responses |
| Frontend | Manual | Visual verification of memory browser and suggestions page |

---

## Dependencies

- No new Go dependencies needed (pgvector queries use raw SQL via GORM, BM25 uses built-in PostgreSQL tsvector).
- Vector embeddings will be generated by the existing `pkg/llm` gateway (embed endpoint).
- No frontend npm additions needed (uses existing Lucide React icons + Tailwind).
