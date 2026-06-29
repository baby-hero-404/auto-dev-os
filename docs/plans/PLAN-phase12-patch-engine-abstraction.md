# Patch Engine Abstraction — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Refactor `server/internal/orchestrator/patch` to decouple the patch application logic into a pluggable `PatchEngine`. Implement a strict `PatchValidator` and introduce the `SearchReplaceApplier` to eliminate the fragility of `git apply` and recover gracefully from LLM patch hallucinations.

**Architecture:** The patch application process transitions from a single `ApplyPatch` function to a `PatchEngine` interface. Strategies (Unified Diff, Search & Replace) are injected. Before application, `Validator` strictly checks metadata. Workflow steps (`code_backend`, `code_frontend`, `fix`) are updated to rely solely on the Engine.

**Tech Stack:** Go 1.23, `testing` stdlib, `regexp` package.

---

## Current Problems

| Problem | Location | Impact |
| :--- | :--- | :--- |
| **No Validation Firewall** | `patch/applier.go` | Malformed patches are fed directly to Git, causing opaque `exit code 2` fatal errors. |
| **Monolithic Apply Logic** | `patch/applier.go` | Cannot easily swap or fallback to alternative editing strategies (e.g., Search & Replace). |
| **Mathematical Fragility** | `git apply` binary | LLMs hallucinates hunk headers; Git rejects perfectly good code replacements due to mathematical line-count mismatch. |
| **Fatal Exit on Apply** | `steps/code_backend.go` | Failing to apply a patch terminates the workflow, preventing the AI from fixing its own mistake in the Review step. *(Mitigated short-term, requires permanent architectural fix).* |

---

## Migration Shape

```text
1. Build Validation Layer (Syntax, Metadata, Uniqueness)      → Task 1
2. Implement Search & Replace Strategy (Aider State Machine)  → Task 2
3. Define PatchEngine Interface & Factory                     → Task 3
4. Refactor Workflow Steps to use PatchEngine + In-Step Retry → Task 4
```

---

## Task 1: Build Validation Layer (`PatchValidator`)
 
> Addresses: The need for a strict firewall before executing destructive system operations.
 
**Files:**
- Create: `server/internal/orchestrator/patch/validator.go`
- Create: `server/internal/orchestrator/patch/validator_test.go`
 
- [x] **Step 1: Define `ValidationError` struct**
  ```go
  type ValidationError struct {
      RepoName string
      Filepath string
      Reason   string
      IsFatal  bool
  }
  ```
- [x] **Step 2: Implement `ValidateUnifiedDiff(patch string) []ValidationError`**
  - Parse hunk headers (`@@ -R,C +R,C @@`).
  - Read target file from disk.
  - Assert that line counts match the expected state.
- [x] **Step 3: Implement `ValidateSearchReplace(blocks []EditBlock) []ValidationError`**
  - Verify that each extracted `SEARCH` block matches exactly once in the target file.
  - If a search block matches multiple times, emit an ambiguous match error.
  - If a search block doesn't match at all, emit a mismatch error.
- [x] **Step 4: Write comprehensive unit tests**
  - Test valid diffs.
  - Test mismatched hunk headers.
  - Test duplicate search blocks and non-matching search blocks.
- [x] **Step 5: Run tests and commit**
  ```bash
  go test ./internal/orchestrator/patch/ -run "TestValidator" -v
  git commit -m "feat(orchestrator/patch): introduce strict patch validation firewall"
  ```
 
---
 
## Task 2: Implement Search & Replace Strategy (Aider Pattern)
 
> Addresses: The fragility of Unified Diffs.
 
**Files:**
- Create: `server/internal/orchestrator/patch/search_replace.go`
- Create: `server/internal/orchestrator/patch/search_replace_test.go`
 
- [x] **Step 1: Implement State Machine Parser**
  - Design a line-by-line parser state machine in `search_replace.go`.
  - State transitions: `StateNormal` -> (reads `<<<<<<< SEARCH`) -> `StateSearch` -> (reads `=======`) -> `StateReplace` -> (reads `>>>>>>> REPLACE`) -> `StateNormal`.
  - Populate slice of `EditBlock` containing the target filepath, search block, and replace block.
- [x] **Step 2: Implement independent Validator**
  - Validate that files exist.
  - Ensure search content exists exactly once in the target file (calls Task 1 validator).
- [x] **Step 3: Implement Applier Logic**
  - Perform safe `strings.Replace` on target file in memory.
  - Flush modified file back to disk.
- [x] **Step 4: Write Unit Tests**
  - Test parsing with standard Markdown block wrapping (e.g. diff block syntax).
  - Test multi-block edits on a single file.
  - Test error handling for multiple matches or missing files.
- [x] **Step 5: Run tests and commit**
  ```bash
  go test ./internal/orchestrator/patch/ -run "TestSearchReplace" -v
  git commit -m "feat(orchestrator/patch): implement search and replace parser, validator, and applier"
  ```
 
---
 
## Task 3: Define `PatchEngine` Interface & Factory
 
> Addresses: Decoupling workflow steps from the monolithic `git apply` applier.
 
**Files:**
- Modify/Create: `server/internal/orchestrator/patch/engine.go`
- Modify: `server/internal/orchestrator/patch/applier.go` (Legacy Git Applier)
 
- [x] **Step 1: Define `PatchEngine` Interface**
  ```go
  type PatchEngine interface {
      // Validate checks structural integrity before application.
      Validate(patchData string, workspace *models.TaskWorkspace) []ValidationError
      
      // Apply executes the patch strategy.
      Apply(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error
  }
  ```
- [x] **Step 2: Implement Factory `NewEngine(preferredStrategy string)`**
  - If `preferredStrategy == "search_replace"`, return `SearchReplaceApplier`.
  - If `preferredStrategy == "unified_diff"` (or default), return `LegacyGitApplier`.
- [x] **Step 3: Refactor legacy `applier.go` to implement `PatchEngine`**
- [x] **Step 4: Run tests and commit**
  ```bash
  go test ./internal/orchestrator/patch/... -v
  git commit -m "refactor(orchestrator/patch): abstract logic into PatchEngine interface"
  ```
 
---
 
## Task 4: Refactor Workflow Steps & Add In-Step Retry
 
> Addresses: Connecting the engine and enabling fast format-level self-healing.
 
**Files:**
- Modify: `server/internal/orchestrator/steps/code_backend.go`
- Modify: `server/internal/orchestrator/steps/code_frontend.go`
- Modify: `server/internal/orchestrator/steps/fix.go`
 
- [x] **Step 1: Update coding steps with an In-Step Retry loop**
  - Implement a retry loop (limit: 2-3 attempts) directly inside each step's execution code.
  - When LLM outputs a patch:
    1. Parse and run `engine.Validate(patch, workspace)`.
    2. If `Validate` returns errors:
       - Format the validation errors into a feedback prompt (e.g. detailing which files/lines failed validation).
       - Prompt the LLM agent again with this feedback to request a corrected patch.
       - Increment the retry counter and re-run.
    3. If `Validate` passes, execute `engine.Apply`.
- [x] **Step 2: Handle terminal validation failure**
  - If validation still fails after max retries:
    - Bypasses step application.
    - Returns `StepResult` with state `Warning` and logs the final validation errors to `PatchApplyError`.
- [x] **Step 3: Define Workflow-Level Retry boundary**
  - Logical bugs, test suite failures, or reviewer rejections must bypass in-step retries and trigger a separate workflow DAG step (e.g. `fix` step scheduled by the engine).
- [x] **Step 4: Run full integration tests**
  ```bash
  go test ./internal/orchestrator/steps/... -v
  git commit -m "refactor(orchestrator/steps): integrate PatchEngine and in-step retry self-healing loop"
  ```
 
---
 
## Verification Checklist
 
- [x] `PatchValidator` correctly rejects `SEARCH` blocks that match >= 2 times.
- [x] `PatchValidator` correctly rejects unified diffs with bad line counts.
- [x] `SearchReplaceApplier` state-machine parser handles markdown backticks and extracts blocks correctly.
- [x] `SearchReplaceApplier` successfully alters a file without executing `git`.
- [x] All 3 coding steps (`code_backend`, `code_frontend`, `fix`) correctly catch validation errors and retry up to 3 times before warning.
- [x] Workflow-level retry is correctly triggered for test suite errors (not formatting errors).
- [x] Legacy unified diff functionality (`git apply`) remains intact for backwards compatibility.
- [x] `go vet ./internal/orchestrator/patch/...` reports no issues.
 
---
 
## Risk Mitigation
 
| Risk | Mitigation |
| :--- | :--- |
| **Search/Replace Fuzzy Match causing bad edits** | Enforce **Strict String Matching** initially. Fuzzy matching (whitespace/indentation ignoring) will only be enabled if exact match fails and confidence is 100%. |
| **Breaking existing agents** | Default strategy returned by factory remains `unified_diff`. Search & Replace is opt-in via a configuration flag until fully stabilized. |
| **Infinite LLM Loop on failure** | The in-step retry loop has a strict hard limit of 3 attempts. After 3 failures, the step gracefully outputs a warning status without hanging the queue. |

