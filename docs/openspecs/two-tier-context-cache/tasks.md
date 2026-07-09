# Tasks: Two-Tier Context Cache Architecture

## P0 — Critical

### Task 1.1: Refactor ContextEngine for Two-Tier Initialization
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Modify `provider.NewProvider` to accept an explicit `workspace_cache.db` path instead of a global path.
- [x] Implement a function to fast-copy `global_cache_<repo>_<commit>.db` to `workspace_cache.db` during `ContextLoadStep`.
- [x] Update `RetrieveContext` to ensure queries are strictly routed to the local DB instance.

### Task 1.2: Implement Lazy Fallback Cache Builder
> Links to: REQ-002

**Acceptance Criteria:**
- [x] Create logic in `ContextLoadStep` to check for the existence of the required global cache file.
- [x] If missing, trigger a synchronous indexing process that writes to the global cache directory before proceeding with the file copy.

## P1 — High

### Task 2.1: Implement GlobalCachePrewarmer Worker
> Links to: REQ-001

**Acceptance Criteria:**
- [x] Create a background worker (Cronjob polling every 15 minutes) that checks for new commits on the configured default integration branch of active repositories.
- [x] When a new commit is detected, clone it to a temporary directory, index it, and move the resulting DB to the global cache directory.

### Task 2.2: Implement CacheGarbageCollector
> Links to: REQ-003

**Acceptance Criteria:**
- [x] Create a cronjob worker that runs daily to scan the global cache directory.
- [x] Implement Reference Counting: check active tasks in the database; do not delete any global cache referenced by an active task.
- [x] Delete global cache files older than 7 days that are no longer referenced.

## P2 — Medium
(none)

## P3 — Low
(none)
