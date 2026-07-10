# PLAN: Phase 20 - AgentVFS Path Isolator (Contextual Path Manager)

## Objective
Currently, the Orchestrator's `ContextEngine` and `PatchEngine` use the absolute/workspace-relative paths (e.g., `code/repos/tool_zentao/worktrees/backend/internal/main.go`). This causes **Path Leakage**, consuming excessive LLM tokens, confusing the AI, and triggering hallucinations (like creating a physical `b/` artifact).

The goal of this phase is to implement **Option A (AgentVFS)**: A strict, sandbox-like `AgentPathContext` that isolates each Agent to its specific worktree Git Root. The LLM will only ever see clean, relative paths (`internal/main.go`).

## Scope of Changes

### 1. `server/pkg/paths/agent_vfs.go` (NEW)
Create the core VFS middleware struct that manages paths for an assigned Agent.
- **`type AgentPathContext struct`**: Wraps the physical absolute path of the specific worktree.
- **`func (v *AgentPathContext) ToLogical(physical string) (string, error)`**: Translates `.../worktrees/backend/main.go` -> `main.go`.
- **`func (v *AgentPathContext) ToPhysical(logical string) (string, error)`**: Translates `main.go` -> `.../worktrees/backend/main.go`. Must include strict `filepath.Clean()` validation to reject paths escaping the worktree (e.g., `../secrets.txt`).
- **`func (v *AgentPathContext) StripGitDiffArtifacts(path string) string`**: Automatically strip `a/` and `b/` prefixes if they bleed into the LLM output.

### 2. `server/internal/context/provider/provider.go`
Update the `ContextEngine` to leverage the `AgentPathContext`.
- Replace the legacy logic that uses the workspace `rootDir` for `filepath.Rel`.
- When scanning the repository, find the true **Git Root**.
- Filter all output paths through `AgentPathContext.ToLogical()` before sending them to the LLM via `GetRepoMap` and `RetrieveContext`.

### 3. `server/internal/patch/engine.go` (or `patcher.go`)
Update the Patch Engine to translate LLM-generated paths back to physical ones safely.
- When parsing AST or Diff payloads from the LLM, pass every file path through `AgentPathContext.ToPhysical()`.
- Explicitly block any `Unauthorized File Modification` exceptions thrown by `ToPhysical`.

### 4. `server/internal/orchestrator/prompts/assembler.go` & Execution Steps
Update `code_backend.go` and `code_frontend.go`.
- During initialization of the LLM prompt, instantiate `paths.NewAgentPathContext(worktreeDir)`.
- Pass this Context into the LLM instruction assembler and the Patch Engine so both use the exact same isolator.
- Remove hardcoded string logic in `code_backend.go` (e.g. `Your diff paths MUST include the repository name prefix`).

## Execution Constraints
- **Multi-Repo Check**: If the task has multiple repos (e.g., a `Review` agent scanning both frontend and backend), the `AgentPathContext` should fallback to `repo_name/file_path` prefixing (Option B hybrid).
- **Concurrency**: Must not use `os.Chdir()`. All path translations must be purely in-memory string manipulations via `filepath` package to remain thread-safe.

## Acceptance Criteria
- [x] LLM Prompts (RepoMap) only display clean paths (e.g. `main.go` instead of `code/repos/...`).
- [x] Generating Git Diff strings using `a/main.go` and `b/main.go` successfully patches without creating actual `b/` folders.
- [x] Attempting to patch files outside the designated worktree returns a strict `PolicyViolationError`.
- [x] Unit tests for `AgentPathContext` handling `../` and absolute path attacks pass.

## Implementation Status

- **Status**: Completed
- **Completion Date**: 2026-07-08
- **Details**:
  - Refactored `PatchEngine` validation to accept `context.Context` to access context-bound path isolation configurations.
  - Implemented boundary validations against `AgentPathContext` in both `LegacyGitApplier` and `SearchReplaceApplier`.
  - Ensured physical file modifications translate correct paths via `AgentPathContext.ToPhysical()`, including edge cases where the base path is empty or direct absolute paths are provided.
  - Verified system stability against all orchestrator package unit tests.
