# Proposal: Two-Tier Context Cache Architecture

## Why
Currently, the ContextEngine uses a single shared SQLite database for AST tags, leading to cross-workspace context leakage (Agent reads unmerged code from other tasks). If we isolate the cache per workspace to solve this, we introduce a severe performance penalty where every new task cloning a large repository must re-index from scratch. We need a Two-Tier Cache Architecture to guarantee absolute task isolation without sacrificing cold-start performance.

## What Changes
### Issue 1: Context Isolation
- Introduce local `workspace_cache.db` for each task workspace to isolate incremental modifications.
- Remove direct queries to the global `cache.db` from within the task's context retrieval flow.

### Issue 2: Cold-Start Performance
- Implement a Global Base Cache (`global_cache_<repo>_<commit>.db`) indexed by repository and commit hash.
- Implement a cache copy mechanism during `ContextLoadStep` to instantiate the local workspace cache from the global base cache.

### Issue 3: Automated Cache Building & Garbage Collection
- Introduce an event-driven Background Worker to pre-build the Global Base Cache via Webhook or a 15-minute Cronjob.
- Implement a Lazy Fallback mechanism for cache misses.
- Implement a Garbage Collection (GC) job to prune outdated global caches using Reference Counting and TTL.

## Capabilities
### New Capabilities
- `GlobalCachePrewarmer`: Background job to build global AST cache for new commits on target branches.
- `CacheGarbageCollector`: Prunes outdated global cache files safely.

### Modified Capabilities
- `ContextEngine.IndexWorkspace`: Now separates global base indexing from local incremental indexing.
- `ContextEngine.RetrieveContext`: Queries only from the local `workspace_cache.db`.

### Removed Capabilities
- N/A

## Impact
| Area | Files Affected |
|------|----------------|
| Context Provider | `server/internal/context/provider/provider.go` |
| Orchestrator Steps | `server/internal/orchestrator/steps/context_load.go` |
| Background Workers | `server/cmd/api/main.go` |
| Background Workers | `server/internal/orchestrator/workers/cache_prewarmer.go` (new) |
| Background Workers | `server/internal/orchestrator/workers/cache_pruner.go` (new) |
