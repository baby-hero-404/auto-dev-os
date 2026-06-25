# Clean Code Audit & Architecture Report: `server/internal/orchestrator`

This document performs a comprehensive code quality audit and architecture review of the `server/internal/orchestrator` directory. It evaluates compliance with clean coding principles (SRP, DRY, KISS, Go idioms) and feature requirements documented in `docs/features/5.6-task-system.md` and `docs/features/5.7-workflow-engine.md`.

---

## 1. Hotspots & Architectural Evaluation

### 1.1. Single Responsibility Principle (SRP) Violations
*   **The "God File" (`orchestrator_steps.go`)**:
    At 55KB+, this file defines step runners for the entire workflow lifecycle. Defining these runners as anonymous functions within a map closure leads to high mock/setup costs and tight closure coupling, making isolated unit testing of steps difficult (even though they can technically be called from the stepRunners map).
    *   *Recommendation*: Refactor the step handlers into discrete components/structs implementing a unified `StepExecutor` interface:
        ```go
        type StepExecutor interface {
            Execute(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) (map[string]any, error)
        }
        ```
        Move each step definition into its own file (e.g., `step_analyze.go`, `step_merge.go`).

### 1.2. Don't Repeat Yourself (DRY) & Command Separation
*   **Inline Git/Shell Execution**:
    Multiple step functions construct shell commands using `fmt.Sprintf` and pass them to `o.runSandboxStep`. The orchestrator should not have hardcoded git command structures like:
    `git -C %s checkout %s` or `git -C %s diff --name-only --diff-filter=U`.
    *   *Recommendation*: Abstract git-specific sandbox operations into the `GitOpsClient` interface or a specialized `SandboxGitClient` helper to segregate sandbox shell scripting from high-level workflow state coordination.

### 1.3. KISS & Nested Conditional Loops (Hotspot: `StepMerge`)
*   **Deep Nesting**:
    `StepMerge` contains complex nesting and error checks for backend and frontend branches. The logic checks whether a branch exists, executes the merge, checks for conflicts, aborts if conflicted, and updates status database models across multiple repositories.
    *   *Recommendation*: Extract the branch merge logic into a clean helper method:
        ```go
        func (o *Orchestrator) mergeBranchForRepo(ctx context.Context, task *models.Task, agent *models.Agent, repo models.Repository, branchName string) (models.MergeStatus, error)
        ```
        This isolates git concerns and leaves the orchestrator loops clean.

### 1.4. Go Error Handling & Swallowing
*   **Swallowed Operations**:
    Several operations (such as saving artifacts, updating workspace metadata, or pruning logs) use blank identifiers `_ =` to swallow errors on write operations.
    *   *Recommendation*: Log errors at the `warn` level even if they are non-fatal, preventing quiet degradation of logging or tracking state:
        ```go
        if err := o.saveArtifact(ctx, jobID, task.ID, step, name, data); err != nil {
            o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to save artifact %s: %v", name, err))
        }
        ```

---

## 2. Alignment with Feature Specifications

### 2.1. Multi-Repo Workspace Isolation
*   **Status**: Mostly compliant with path-boundary hardening. The implementation successfully parses repository configurations inside `metadata.json` and manages isolated worktrees under `code/repos/`. Substring matching in `FindRepoWorkspaceByPath` presented a path-boundary matching risk (e.g. `api` matches `api-client`), which requires hardening to use `filepath.Rel` with exact boundary checks.
*   **Security Lock & Panic Recovery**: While workspace files utilize DB advisory and local lock files, the orchestrator runner lacks a recovery wrapper around workflow execution. A panic during execution will bypass the cleanup block, leaving workspace lock files and DB advisory locks unreleased.

### 2.2. Review-Fix Loop Policies
*   **PR Rejection Feedback Policy**: Handled via `CheckReviewLoopLimit` in `server/internal/handler/pr.go:93`. When a human reviewer rejects a PR, this policy runs before the current rejection feedback is saved. It checks the stored cycle count against `MaxReviewFixCycles`, blocking further rejection/fix cycles after the stored rejection count reaches the limit (e.g. with `MaxReviewFixCycles = 3`, it allows the third rejection and halts/fails on the next rejection attempt).
*   **Internal Agent Review Loop Policy**: Located in the main runner's internal loop (`server/internal/orchestrator/orchestrator_steps.go:991`). When the internal review fails, it loops but does not halt with a fail status when the cycles limit is reached; instead, it proceeds to the testing step with a `cycle_limit_reached` flag.

---

## 3. Refactoring Case Studies

### 3.1. Case Study: Refactoring `StepMerge`
The original logic in `StepMerge` uses nested conditional branches to handle backend and frontend integrations. Here is the proposed pseudo-code abstraction version (assuming git helper routines such as `gitCheckout`, `gitBranchExists`, `gitMergeBranch`, and `gitCommitMerge` are implemented as part of the orchestrator or a Git client helper):

```go
// Proposed refactoring to extract step tasks and enforce guard clauses

func (o *Orchestrator) mergeRepositoryBranches(ctx context.Context, task *models.Task, agent *models.Agent, repo models.Repository, ws *models.TaskWorkspace, beBranch, feBranch, integrationBranch string) (models.MergeStatus, error) {
	localPath := o.repoHostPath(task, ws, repo)
	containerPath := o.containerPathForHostPath(task, localPath, "")

	// 1. Checkout integration branch
	if err := o.gitCheckout(ctx, task, agent, containerPath, integrationBranch); err != nil {
		return models.MergeStatusFailed, fmt.Errorf("checkout integration failed: %w", err)
	}

	// 2. Merge backend branch if present
	if hasBe, _ := o.gitBranchExists(ctx, task, agent, containerPath, beBranch); hasBe {
		if status, err := o.gitMergeBranch(ctx, task, agent, containerPath, beBranch); err != nil {
			return status, err
		}
	}

	// 3. Merge frontend branch if present
	if hasFe, _ := o.gitBranchExists(ctx, task, agent, containerPath, feBranch); hasFe {
		if status, err := o.gitMergeBranch(ctx, task, agent, containerPath, feBranch); err != nil {
			return status, err
		}
	}

	// 4. Commit merge changes
	if err := o.gitCommitMerge(ctx, task, agent, containerPath); err != nil {
		return models.MergeStatusFailed, fmt.Errorf("merge commit failed: %w", err)
	}

	return models.MergeStatusMerged, nil
}
```

### 3.2. Case Study: Safe Path Resolving in Grep Search
Using safe paths prevents symlink/path escape for walked files in grep search. We transitioned the unsafe file walk read into a bounded resolver.

*Before:*
```go
data, err := os.ReadFile(path)
```

*After (Refactored & Secure):*
```go
safePath, err := resolveSafePath(workspaceRoot, rel)
if err != nil {
    return nil // skip unsafely evaluated files or traversal attempts
}
data, err := os.ReadFile(safePath)
```
