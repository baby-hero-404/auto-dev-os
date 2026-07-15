# Specs: Review→Fix Seam Hardening 2026

## Added Requirements

### REQ-001: Fix step advertises the same toolset it enforces
> ❌ Status: Not Started

**Scenario:**
- WHEN an agentic step (fix, code_backend, code_frontend, review) assembles its LLM tool definitions
- THEN the role used for `ToolsForRole` is the same effective role used by the boundary-checked executor at execution time
- AND for the fix step that effective role has `CapEdit` and `CapCreate`, so `search_replace` and `create_file` appear in the advertised tool list
- AND no instruction template names a tool that is absent from the advertised list for that step/role

**Failure scenario (regression from task 8291a25e):**
- WHEN the fix step runs under a `reviewer` or `planner` agent
- THEN it must NOT present a read-only tool list while its instruction demands edits
- AND a tool call that is advertised must never be rejected by `Registry.Execute` for role authorization

### REQ-002: Workspace diffs use repository-relative paths
> ❌ Status: Not Started

**Scenario:**
- WHEN `GetWorkspaceDiff` produces a diff for a repo checked out at `code/repos/<repo>/<branch>/`
- THEN diff headers read `a/<repo-relative-path>` / `b/<repo-relative-path>` (e.g. `a/cmd/sync/main.go`)
- AND repository identity is carried only by the `--- Repository: <name>` section header
- AND `GetWorkspaceChangedFiles` returns the same repo-relative form
- AND `GetDiff`/`GetPRDiff` behave identically (no prefix injection for any task type)

### REQ-003: Review findings are typed and canonicalized before reaching fix
> ❌ Status: Not Started

**Scenario:**
- WHEN the review step parses LLM output into findings
- THEN each finding is decoded into `models.ReviewFinding{Repo, File, Line, Severity, Recommendation}` where `File` is repository-relative by contract
- AND before the fix instruction is built, the runtime canonicalizes every `File`: strips a leading `code/repos/<repo>/<branch>/` prefix if present, collapses any duplicated prefix, and cleans the path
- AND a finding whose path still cannot be resolved inside the repository is dropped from the fix instruction and logged at warn level (it must not poison the prompt)
- AND the fix instruction header states: paths are repository-relative to repository `<name>`

**Failure scenario (regression from call-131):**
- WHEN a reviewer emits `"file": "code/repos/tool_zentao/main/cmd/sync/main.go"`
- THEN the fix prompt contains `cmd/sync/main.go`
- AND no tool call issued from that prompt can create `code/repos/tool_zentao/main/code/repos/tool_zentao/main/...`

### REQ-004: Tool layer rejects self-nested repository paths
> ❌ Status: Not Started

**Scenario:**
- WHEN an edit-capability tool receives a path that re-enters the workspace's own repo layout (a `code/repos/<repo>/` segment while the tool workspace root is already a repository checkout)
- THEN the call fails with an actionable error naming the expected repo-relative form (e.g. `Error: path "code/repos/tool_zentao/main/internal/x.go" appears workspace-prefixed; this workspace is the repository root — use "internal/x.go"`)
- AND `patch.EvaluatePolicy` scores such paths as `SeverityError` (fed back to the model), not Warning/auto-expansion
- AND no phantom directory hierarchy is created (`MkdirAll` never runs for a rejected path)

### REQ-005: Fix step runs under a coder persona
> ❌ Status: Not Started

**Scenario:**
- WHEN the fix step assembles its prompt
- THEN the system prompt uses a coder role profile (backend/frontend), never `# Planner Role` ("Do NOT write implementation code") or `# Reviewer Role`
- AND the persona is selected by step semantics, not by which agent owns the workflow stage

### REQ-006: Authorization errors are actionable
> ❌ Status: Not Started

**Scenario:**
- WHEN `Registry.Execute` rejects a tool call for role authorization
- THEN the rejection is returned to the model as an in-loop tool error (loop-correctable — the step does NOT fail fast)
- AND the error lists the tools the current role IS allowed to use (from `ToolsForRole`, the same source as advertisement)
- AND the message states the rejection is permanent for this step, so the model stops retrying it (task 8291a25e: calls 108→127→131 permuted paths against an invisible authorization wall)
- AND pathological repetition remains bounded by the existing circuit breaker and iteration budget (no new fail-fast path is introduced)

## Modified Requirements

### REQ-M01: Fix instruction context header
> ❌ Status: Not Started

**Scenario:**
- WHEN the fix instruction is built (`fix.go`)
- THEN the context line reads that all diff and finding paths are **repository-relative** (replacing today's incorrect "relative to your workspace root")
- AND the diff embedded in the instruction already satisfies REQ-002, so header and content agree

### REQ-M02: Backend/frontend roles can create files
> ❌ Status: Not Started (uncommitted hotfix exists in working tree)

**Scenario:**
- WHEN a coding step runs under the `backend` or `frontend` role
- THEN `create_file` is advertised and authorized (`CapCreate` in `DefaultRoleProfiles`)
- AND the coding instruction's "you MUST use the 'create_file' tool" clause refers to a tool the model actually has

## Removed Requirements
- REQ-R01: Workspace-path prefix injection (`--src-prefix=a/<workspace-rel>/`) in `GetWorkspaceDiff`/`GetWorkspaceChangedFiles`.
- REQ-R02: Untyped `any` findings pass-through from review output to fix instruction (`getReviewFindings` returning `any`).
