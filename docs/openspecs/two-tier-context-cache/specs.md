# Specs: Two-Tier Context Cache Architecture

## ADDED Requirements

### REQ-001: Background Global Cache Pre-warming
> ❌ Status: Not Started

**Scenario: Auto-indexing new commits via cron/webhook**
- **WHEN** a new commit is detected on the configured default integration branch of a registered repository
- **THEN** the `GlobalCachePrewarmer` SHALL automatically trigger the AST indexing process for that commit
- **AND** save the result as a Read-Only SQLite database file `global_cache_<repo_id>_<commit_hash>.db`

### REQ-002: Lazy Fallback Cache Building
> ❌ Status: Not Started

**Scenario: Cache miss during task context load**
- **WHEN** a task starts and requests the base cache for commit `X`, but `global_cache_<repo_id>_X.db` does not exist
- **THEN** the system MUST block the task, build the global cache for commit `X`, save it, and then proceed

### REQ-003: Safe Global Cache Garbage Collection
> ❌ Status: Not Started

**Scenario: Pruning outdated cache files**
- **WHEN** the `CacheGarbageCollector` runs its daily schedule
- **THEN** it SHALL delete any global cache files older than 7 days
- **AND** it MUST NOT delete any cache file that is actively referenced by a running/open task

## MODIFIED Requirements

### REQ-M01: Two-Tier Cache Instantiation
> ❌ Status: Not Started

**Scenario: Initializing local workspace cache**
- **WHEN** a task successfully clones the repository into its workspace
- **THEN** the `ContextLoadStep` MUST copy the corresponding `global_cache_<repo_id>_<commit_hash>.db` to the workspace's local `context/workspace_cache.db`
- **AND** `RetrieveContext` SHALL only query this local `workspace_cache.db`
