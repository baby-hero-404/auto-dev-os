# Proposal: Review→Fix Seam Hardening 2026

## Why

Task `8291a25e` (zentao auto) failed terminally with `structural failure (no workspace progress)` after 6 review→fix cycles and 154 LLM calls (~103 spent in fix loops). The full trace is in `docs/reports/task-8291a25e-nested-path-trace-verification-report.md`. Three verified, code-located defects caused it — none of them model capability:

1. **Fix-step tool starvation.** Tools are advertised from the raw agent role (`llm_step.go:48` → `ToolsForRole(agent.Role)`), but fix runs under `planner`/`reviewer` agents whose profiles have no edit capabilities (`capability.go`). The instruction text simultaneously commands `Use the available tools (e.g. search_replace, create_file)`. The model hallucinated undeclared edit calls; `Registry.Execute` rejected them (`role "reviewer" is not authorized to use tool "create_file"`), every loop burned its 8 iterations on navigation, and `ErrNoProgress` fired — accurately, because zero edits succeeded. The uncommitted hotfix (`llm_step.go:87` remapping executor role `reviewer→backend`) fixes enforcement only: the model now *executes* tools it was never *advertised*.

2. **Workspace-diff prefix injection.** `GetWorkspaceDiff` (`gitops/client.go:195`) runs `git diff --src-prefix=a/code/repos/<repo>/main/`, deliberately rewriting repo-relative paths into workspace-relative ones. The reviewer copies these paths into findings; `fix.go:160` passes them into the fix prompt verbatim while `fix.go:163` claims "All file paths are relative to your workspace root" — but the tool executor resolves against the **repo root** (`resolveAgenticWorkspace`). Result: call-131 `create_file code/repos/tool_zentao/main/internal/...` physically created a nested repo copy inside the repo. The current working-tree hotfix to `getDiffPrefixes` covers `GetDiff`/`GetPRDiff` only — the multi-repo Python path used by review/fix still injects prefixes.

3. **No path canonicalization or duplicate-prefix guard anywhere.** Review findings cross the review→fix seam as untyped `any` (`review.go:38`), `SafeWorkspacePath` only blocks traversal *escaping* the root, and `patch.EvaluatePolicy` scored the nested path as a mere Warning (auto-expanded boundary). Nothing in prompt-builder, boundary policy, or tool layer detects a duplicated `code/repos/<repo>/<branch>/` prefix.

Secondary verified defects: fix runs under Planner ("Do NOT write implementation code") or Reviewer personas, never a coder persona; and tool-authorization errors are not actionable, causing path-permutation thrashing (calls 108→127→131).

## What Changes

### Issue 1: Fix-step toolset and role alignment
- Introduce a single `effectiveRoleForStep(stepID, agentRole)` resolution used by **both** tool advertisement (`ToolsForRole`) and the boundary-checked executor, replacing the executor-only remap at `llm_step.go:87`.
- Commit the working-tree hotfix adding `CapCreate` to `backend`/`frontend` role profiles (`capability.go`) — during the logged run, coding steps had `search_replace` but no `create_file`, while the coding instruction mandates `create_file` for new files.
- Make `Registry.Execute` authorization errors actionable: state which tools ARE available to the current role instead of only naming the rejected one.

### Issue 2: Repo-relative workspace diffs
- Remove the `--src-prefix`/`--dst-prefix` workspace-path injection from the `GetWorkspaceDiff`/`GetWorkspaceChangedFiles` Python scripts (`gitops/client.go:159-250`); keep repository attribution in the existing `--- Repository: <name>` header line, which already carries it.
- Complete (and commit) the `getDiffPrefixes` bypass so `GetDiff`/`GetPRDiff` behavior is consistent with the multi-repo path.
- Fix the fix-instruction context line (`fix.go:163`) to declare paths **repository-relative** and name the repository.

### Issue 3: Typed, canonicalized review→fix seam
- Replace the `any` pass-through (`getReviewFindings`, `review.go:38`) with a typed `models.ReviewFinding{Repo, File, Line, Severity, Recommendation}` where `File` is defined as repository-relative.
- Add a runtime canonicalization step that normalizes every finding path before it enters the fix instruction: strip known workspace prefixes (`code/repos/<repo>/<branch>/`), collapse duplicates, reject paths that still fail validation.
- This is the first concrete consumer of the `execution-semantics-2026` typed-contract philosophy applied to the seam that actually failed.

### Issue 4: Tool-layer duplicate-prefix guard (defense in depth)
- Extend `tool.SafeWorkspacePath` (or the boundary executor) to detect a path that re-enters the workspace's own repo layout (`code/repos/<repo>/…` when the workspace root is already a repo checkout) and reject it with an actionable error, instead of `MkdirAll`-ing a phantom hierarchy.
- Upgrade `patch.EvaluatePolicy` handling of such paths from Warning/auto-expansion to Error.

### Issue 5: Fix-step persona
- Run the fix step under a coder persona (backend/frontend prompt profile), not the Planner/Reviewer persona of the agent that happens to own the workflow stage.

## Capabilities

### New Capabilities
- `models.ReviewFinding` typed contract with repository-relative path semantics.
- Path canonicalizer for finding/diff paths (`paths` package).
- Duplicate-repo-prefix detection in the tool path-resolution layer.

### Modified Capabilities
- Tool advertisement/enforcement role resolution (shared `effectiveRoleForStep`).
- `backend`/`frontend` role profiles gain `CapCreate`.
- `GetWorkspaceDiff` / `GetWorkspaceChangedFiles` emit repo-relative paths.
- Fix-step instruction header and persona.
- `Registry.Execute` authorization error message.

### Removed Capabilities
- Workspace-path prefix injection in multi-repo diff generation.
- Untyped `any` findings pass-through between review and fix.

## Impact

| Area | Files Affected |
|------|----------------|
| Orchestrator step wiring | `server/internal/orchestrator/llm_step.go` |
| Role capabilities | `server/internal/tool/capability.go` |
| Tool registry errors | `server/internal/tool/registry.go` |
| Diff generation | `server/internal/orchestrator/gitops/client.go` |
| Review step | `server/internal/orchestrator/steps/review.go` |
| Fix step | `server/internal/orchestrator/steps/fix.go` |
| Models | `server/pkg/models/task.go` (ReviewFinding) |
| Path helpers | `server/pkg/paths/helpers.go`, `server/internal/tool/helpers.go` |
| Boundary policy | `server/internal/orchestrator/patch/` (EvaluatePolicy), `server/internal/orchestrator/steps/boundary_tool_executor.go` |
