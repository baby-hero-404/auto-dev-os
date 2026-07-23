# Tasks: Simplify Repository Workspace Paths

## P0 — Critical

### Task 1.1: Refactor `pkg/paths` interfaces
> Links to: REQ-M01, REQ-M02

**Acceptance Criteria:**
- [x] Remove `branch` parameter from `WorkspacePaths.RepoMain` and `RepoMainRelative`.
- [x] Remove `branch` parameter from `RepoRelativeToWorkspace`.
- [x] Remove `FindRepoMainBranchDir` completely.
- [x] Update `WorkspaceToRepoRelative` logic if it relied on variable branch handling (ensure it expects `main` or `worktrees`).

### Task 1.2: Update Orchestrator implementations
> Links to: REQ-M01, REQ-R01

**Acceptance Criteria:**
- [x] Update `internal/orchestrator/wkspace/create.go` and `state.go` to stop passing `defaultBranch` to `RepoMainRelative`.
- [x] Update `internal/orchestrator/repoutil/paths.go` and `internal/orchestrator/tester/runner.go` to remove `branch` args.
- [x] Update `internal/orchestrator/patch/diff.go` and `applier.go` to remove `branch` args.
- [x] Update `internal/orchestrator/steps/code_frontend.go` and `code_backend.go` path resolutions.
- [x] Replace any `paths.FindRepoMainBranchDir` calls with a hardcoded `"main"` string if necessary (or just remove the variable entirely).

### Task 1.3: Update Tests
> Links to: REQ-M01, REQ-M02

**Acceptance Criteria:**
- [x] Fix compiler errors in `pkg/paths/workspace_test.go` and other tests.
- [x] Ensure `go test ./...` passes across the entire orchestrator and paths packages.

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
