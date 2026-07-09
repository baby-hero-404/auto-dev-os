# Proposal: Simplify Repository Workspace Paths

## Why
Currently, the workspace path layout dynamically includes the target branch name in the directory structure (e.g., `code/repos/<repo_name>/<branch>`). This creates several points of friction:
- **Path Complexity:** Path parsing utilities, LLM diff processors, and test runners must constantly resolve or guess the branch name to construct the correct absolute or relative path.
- **Fragility in Sandboxes:** Test runners and git appliers (like `patch -p1`) become brittle if the branch name unexpectedly changes or diverges.
- **Redundancy:** The branch state is already tracked via git and in the workspace metadata. Reflecting it in the file system path adds unnecessary complexity to the `pkg/paths` interfaces.

By removing the dynamic `branch` parameter and standardizing on a static `main` directory (i.e., `code/repos/<repo_name>/main`), we significantly simplify path normalization, testing, and patch execution.

## What Changes

### Issue 1: Remove dynamic `branch` from path interfaces
- Modify `WorkspacePaths.RepoMain` and `RepoMainRelative` to drop the `branch` parameter.
- Modify `RepoRelativeToWorkspace` to drop the `branch` parameter.

### Issue 2: Refactor Orchestrator logic
- Update `internal/orchestrator/wkspace/state.go` and `create.go` to stop passing branch names to path utilities.
- Update `internal/orchestrator/repoutil/paths.go` and `internal/orchestrator/tester/runner.go` to use the simplified signatures.
- Deprecate or simplify `FindRepoMainBranchDir` if it's no longer needed to guess the branch folder name.

## Capabilities

### New Capabilities
- Deterministic and static workspace paths for the primary checkout.

### Modified Capabilities
- `pkg/paths` interface signatures are simplified.
- Workspace metadata reflects static paths (`code/repos/<repo_name>/main`).

### Removed Capabilities
- The ability to have multiple base checkouts with different branch names side-by-side in `code/repos/<repo_name>/<branch>`. (This is acceptable as isolated work is already done in `worktrees/`).

## Impact

| Area | Files Affected |
|------|----------------|
| Paths Interface | `server/pkg/paths/interfaces.go` |
| Paths Implementation | `server/pkg/paths/workspace.go` |
| Paths Tests | `server/pkg/paths/workspace_test.go` |
| Orchestrator Workspace | `server/internal/orchestrator/wkspace/create.go`, `state.go` |
| Orchestrator RepoUtil | `server/internal/orchestrator/repoutil/paths.go` |
| Orchestrator Steps | `server/internal/orchestrator/steps/code_frontend.go`, `code_backend.go` |
| Orchestrator Tester | `server/internal/orchestrator/tester/runner.go` |
| Orchestrator Patch | `server/internal/orchestrator/patch/diff.go`, `applier.go` |
