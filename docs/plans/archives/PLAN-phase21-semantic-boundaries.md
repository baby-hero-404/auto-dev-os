# PLAN: Phase 21 - Semantic Boundaries & RBAC for Filesystem

## Objective
Replace the rigid `affected_files` whitelist with a **Semantic Boundary (RBAC for Filesystem)** architecture. This enables the LLM to autonomously create helper files, tests, and modify dependencies within approved "Modules" based on explicit "Capabilities", drastically improving the TDD and autonomous coding workflow without sacrificing security.

---

## 1. Schema & Model Updates (`server/pkg/models`)

- [ ] **Deprecate `AffectedFiles`**: Gradually phase out `affected_files` in favor of `execution_boundaries`.
- [ ] **Create `ExecutionBoundary` Struct**:
  ```go
  type ExecutionBoundary struct {
      Module       string   `json:"module"`
      Root         string   `json:"root"`
      Capabilities []string `json:"capabilities"`
  }
  ```
- [ ] **Create `ExpandedBoundary` Struct** (for Audit/Review):
  ```go
  type ExpandedBoundary struct {
      File       string `json:"file"`
      Reason     string `json:"reason"`
      Capability string `json:"capability"`
      Risk       string `json:"risk"` // LOW, MEDIUM, HIGH, CRITICAL
  }
  ```
- [ ] **Update `TaskAnalysis`**: Add `ExecutionBoundaries []ExecutionBoundary` and `ExpandedBoundaries []ExpandedBoundary`.

---

## 2. Prompt Engineering (`server/internal/prompts`)

- [ ] **Update Planner Prompt (`analyze.md`)**:
  - Instruct the Analyzer to stop outputting rigid `affected_files`.
  - Teach the Analyzer to output `execution_boundaries`.
  - Define the standard capabilities for the LLM:
    - `modify_existing`: Can modify existing files in the module.
    - `create_helper`: Can create `utils.go`, `types.ts`, `errors.go` in the module.
    - `create_test`: Can create `*_test.go`, `*.spec.ts` in the module.
    - `modify_exports`: Can modify `index.ts`, `mod.rs`, `__init__.py`.
    - `add_dependency`: Can modify `go.mod`, `package.json`, `Cargo.toml`.
    - `generate_mock`: Can generate mock files.
- [ ] **Update Coder Prompt (`coder.md`)**:
  - Feed the capabilities into the Coder's system prompt so it knows its exact filesystem rights.

---

## 3. Patcher Core Refactoring (`server/internal/orchestrator/patch`)

- [ ] **Create `SemanticValidator` Module**:
  - Replace the old `MatchAffectedFile` logic.
  - For each file in the LLM's patch, determine the action (`Create`, `Modify`, `Delete`).
  - Map the action and file path to a required Capability.
    - *Example*: Modifying `go.mod` -> requires `add_dependency`.
    - *Example*: Creating `helper.go` inside `root` -> requires `create_helper`.
- [ ] **Implement Risk Evaluation**:
  - `LOW`: Helpers, Tests, existing files within the module.
  - `MEDIUM`: Config files (`go.mod`, `package.json`).
  - `HIGH`: Dockerfiles, CI/CD scripts (Reject unless explicitly approved).
  - `CRITICAL`: `.github/workflows` (Reject always).
- [ ] **Boundary Expansion Logging**:
  - When the Patcher auto-allows a file via Semantic Boundaries, it generates an `ExpandedBoundary` struct.
  - Accumulate these expansions during `ApplyPatch`.

---

## 4. Orchestrator Workflow Integration (`server/internal/orchestrator/steps`)

- [ ] **Update `analyze.go`**:
  - Parse `execution_boundaries` from the LLM's JSON output.
  - Perform fallback mapping from old `affected_files` to `execution_boundaries` for backward compatibility.
- [ ] **Update `code_backend.go` / `code_frontend.go` / `fix.go`**:
  - Capture the `ExpandedBoundaries` returned by the Patcher.
  - Save the expansion logs to the Task's state (`updateTaskAnalysis`) so they persist in DB.
- [ ] **Update Artifacts / UI Payloads**:
  - Emit the `ExpandedBoundaries` to `TasksMD` or artifact logs so the Frontend Review UI can visually display the expansion reason (e.g., "Auto Expanded - create_helper - LOW risk").

---

## 5. Testing & Validation

- [ ] **Unit Tests for SemanticValidator**:
  - Test that `create_test` allows `sqlite_test.go` but rejects `auth_test.go` (outside module).
  - Test that `add_dependency` allows `go.mod` but rejects `helper.go`.
- [ ] **E2E Task Execution**:
  - Run a mock task through `Analyze` -> `CodeBackend` to ensure the Patcher correctly processes capability-based boundaries and rejects invalid ones.

---

## Summary
This refactoring shifts Auto Code OS from an ACL (Access Control List) mindset to an RBAC (Role-Based Access Control) mindset for the filesystem. It creates a highly resilient TDD environment where Agents can autonomously structure their code (helpers, tests, mocks) safely.
