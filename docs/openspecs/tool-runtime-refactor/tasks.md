# Tasks: Tool Runtime Refactor

## P0 — Critical (Foundation) ✅

### Task 1.1: Define Core Tool Interface & Types ✅
> Links to: REQ-001, REQ-004

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tool.go` with `Tool` interface: `Name()`, `Description()`, `Schema()`, `Category()`, `Capabilities()`, `Execute()`
- [x] Define `Category` constants: `filesystem`, `editing`, `git`, `search`, `build`, `context`, `documentation`
- [x] Define `Capability` constants: `read`, `edit`, `create`, `search`, `build`, `git`, `git.diff`, `context`, `docs`, `dependency`
- [x] Define `Call` struct: `Input`, `Workspace`, `TaskID`, `AgentID`, `AgentRole`
- [x] Define `Result` struct: `Success`, `Message`, `Output`, `FilesChanged`, `Diagnostics`, `Metadata`
- [x] Define `Diagnostic` struct: `Severity`, `File`, `Line`, `Message`
- [x] Unit test: verify `Result` JSON marshaling with all fields populated

### Task 1.2: Implement Tool Registry ✅
> Links to: REQ-002

**Acceptance Criteria:**
- [x] Create `server/internal/tool/registry.go` with `Registry` struct
- [x] `Register(tool)` adds tool to map; panics on duplicate name
- [x] `Execute(ctx, name, call)` dispatches to correct tool; returns error for unknown tool
- [x] `Definitions()` returns `[]llm.ToolDefinition` for all registered tools
- [x] `ToolsForCapabilities(caps)` returns filtered definitions matching any provided capability
- [x] Thread-safe via `sync.RWMutex`
- [x] Unit test: register 3 tools → verify `Definitions()` returns 3 items
- [x] Unit test: `Execute()` with unknown name → returns structured error
- [x] Unit test: `ToolsForCapabilities([CapRead])` → returns only read-capable tools

### Task 1.3: Implement Capability Manager & Role Profiles ✅
> Links to: REQ-003

**Acceptance Criteria:**
- [x] Create `server/internal/tool/capability.go` with `CapabilityManager` and `RoleProfile`
- [x] `DefaultRoleProfiles()` returns profiles for: planner, backend, frontend, reviewer, qa, security-auditor
- [x] `ToolsForRole(role)` resolves role → capabilities → filtered tool definitions
- [x] Unknown role falls back to `[CapRead, CapSearch]`
- [x] Unit test: `ToolsForRole("backend")` includes edit tools, excludes docs-only tools
- [x] Unit test: `ToolsForRole("reviewer")` excludes edit tools
- [x] Unit test: `ToolsForRole("unknown_role")` returns read + search tools only

### Task 1.4: Implement `read_file` Tool (Enhanced) ✅
> Links to: REQ-006

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/read_file.go` implementing `tool.Tool`
- [x] Parameters: `path` (required), `start_line`, `end_line`, `around_line`, `radius`, `max_lines` (default 500)
- [x] Full file read: returns entire content up to `max_lines`
- [x] Line range: `start_line=50, end_line=80` returns only those lines
- [x] Around line: `around_line=150, radius=30` returns lines 120-180
- [x] Result metadata includes `total_lines`, `returned_lines`, `start_line`, `end_line`
- [x] Path validation via `SafeWorkspacePath()` — rejects directory traversal
- [x] Category: `filesystem`, Capabilities: `[CapRead]`
- [x] Unit test: full file read with max_lines truncation
- [x] Unit test: line range read
- [x] Unit test: around_line read
- [x] Unit test: path traversal rejection

### Task 1.5: Implement `search_replace` Tool (Transactional) ✅
> Links to: REQ-005, REQ-011

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/search_replace.go` implementing `tool.Tool`
- [x] Parameters: `path`, `search`, `replace` (required); `dry_run`, `verify` (optional, default false/true)
- [x] Validates search block exists exactly once in file
- [x] Returns `hash_before`, `hash_after`, `replaced_count` in metadata
- [x] Dry-run mode: returns diff preview without modifying file
- [x] Verification pipeline: runs format hook → compile check after successful edit
- [x] Rollback: if verification fails, restores original file content
- [x] Result includes `FilesChanged` on success
- [x] Category: `editing`, Capabilities: `[CapEdit]`
- [x] Unit test: successful search/replace with hash verification
- [x] Unit test: search block not found → error, file unchanged
- [x] Unit test: ambiguous match (2 occurrences) → error, file unchanged
- [x] Unit test: dry-run returns preview without modification
- [x] Unit test: verification failure triggers rollback

### Task 1.6: Implement `grep_search` Tool (Enhanced) ✅
> Links to: REQ-007

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/grep_search.go` implementing `tool.Tool`
- [x] Parameters: `query` (required), `regex` (bool), `include` (glob), `exclude` (glob), `max_results` (default 30)
- [x] Literal mode: uses `strings.Contains` per-line with line numbers
- [x] Regex mode: uses `regexp.Compile` with match extraction
- [x] Output format: `filepath:line_number: matching line content`
- [x] Respects include/exclude globs; always skips `.git`, `node_modules`, `vendor`
- [x] Category: `search`, Capabilities: `[CapSearch]`
- [x] Unit test: literal search returns line numbers
- [x] Unit test: regex search with capture
- [x] Unit test: include/exclude glob filtering

---

## P1 — High (Core Tools) ✅

### Task 2.1: Implement `list_files` Tool ✅
> Links to: REQ-009

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/list_files.go`
- [x] Parameters: `path` (default "."), `max_depth` (default 3), `max_files` (default 200)
- [x] Returns tree structure as formatted text
- [x] Skips `.git`, `node_modules`, `vendor`, `dist`, `build`
- [x] Category: `filesystem`, Capabilities: `[CapRead]`
- [x] Unit test: tree output at depth 2

### Task 2.2: Implement `file_exists` Tool ✅
> Links to: REQ-009

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/file_exists.go`
- [x] Parameters: `path` (required)
- [x] Returns `{exists: bool, size: int, is_dir: bool}` in metadata
- [x] Category: `filesystem`, Capabilities: `[CapRead]`
- [x] Unit test: existing file → true; missing file → false

### Task 2.3: Implement `git_diff` Tool ✅
> Links to: REQ-008

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/git_diff.go`
- [x] Parameters: `staged` (bool, default false), `path` (optional filter)
- [x] Runs `git diff` (or `git diff --staged`) in sandbox
- [x] Returns diff output with `FilesChanged` list
- [x] Category: `git`, Capabilities: `[CapGit, CapGitDiff]`
- [x] Unit test: mock sandbox returns diff → parsed FilesChanged

### Task 2.4: Implement `git_status` Tool ✅
> Links to: REQ-008

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/git_status.go`
- [x] Runs `git status --porcelain` in sandbox
- [x] Parses output into structured `{staged: [], unstaged: [], untracked: []}` metadata
- [x] Category: `git`, Capabilities: `[CapGit]`
- [x] Unit test: parse porcelain output

### Task 2.5: Implement `run_tests` Tool (Enhanced) ✅
> Links to: REQ-010

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/run_tests.go`
- [x] Parameters: `command` (default "go test ./..."), `path` (optional target)
- [x] Runs command in sandbox
- [x] Parses test output into `Diagnostics` (pass/fail per test)
- [x] Category: `build`, Capabilities: `[CapBuild]`
- [x] Unit test: parse Go test output with failures

### Task 2.6: Implement `run_build` Tool ✅
> Links to: REQ-010

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/run_build.go`
- [x] Parameters: `command` (default "go build ./...")
- [x] Runs in sandbox; parses compiler errors into `Diagnostics` with file + line
- [x] Category: `build`, Capabilities: `[CapBuild]`
- [x] Unit test: parse Go compiler error output

### Task 2.7: Implement Verification Pipeline ✅
> Links to: REQ-011

**Acceptance Criteria:**
- [x] Create `server/internal/tool/verify.go` with `VerifyHook` interface and `VerifyPipeline`
- [x] Create `server/internal/tool/verify/gofmt.go` — runs `gofmt -w` on changed `.go` files
- [x] Create `server/internal/tool/verify/compile_check.go` — runs `go build ./...` and parses errors
- [x] Pipeline stops on first error-severity diagnostic
- [x] Unit test: gofmt hook formats file; compile_check detects syntax error

### Task 2.8: Implement Context Tools ✅
> Links to: REQ-012

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/read_spec.go` — resolves `docs/openspecs/<task>/` and returns spec contents
- [x] Create `server/internal/tool/tools/read_affected_files.go` — reads from `task.Analysis.AffectedFiles` with worktree-first resolution
- [x] Category: `context`, Capabilities: `[CapContext]`
- [x] Unit test: read_spec resolves correct files from analysis

---

## P2 — Medium (Integration & Migration) ✅

### Task 3.1: Backward-Compatible SkillExecutor Adapter ✅
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Create `server/internal/tool/adapter.go`
- [x] `SkillExecutorAdapter` wraps `tool.Registry` and implements the old `SkillExecutor.Execute(ctx, SkillCall) SkillResult` signature
- [x] Converts `SkillCall` → `tool.Call`, dispatches to registry, converts `tool.Result` → `SkillResult`
- [x] Existing step code (`code_backend.go`, `code_frontend.go`, `fix.go`) works without modification
- [x] Unit test: adapter converts call and result correctly
- [x] Unit test: adapter handles unknown tool gracefully

### Task 3.2: Migrate Analyze Step Tools ✅
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] `analyzeToolDefinitions()` in `steps/analyze_tools.go` → returns `registry.ToolsForCapabilities([CapRead, CapSearch])`
- [x] `executeAnalyzeTool()` switch → replaced by `registry.Execute(ctx, toolName, call)`
- [x] `listAnalyzeFiles()`, `readAnalyzeFile()`, `grepAnalyzeFiles()` logic moved to `list_files`, `read_file`, `grep_search` tools
- [x] All existing analyze tests pass without modification
- [x] Unit test: analyze step uses registry tools

### Task 3.3: Simplify Prompt Tool Assembly ✅
> Links to: REQ-M03

**Acceptance Criteria:**
- [x] `toolDefinitionsForAgent()` in `prompts/tools.go` → resolves role via `CapabilityManager.ToolsForRole(agent.Role)`
- [x] JIT skill `allowed-tools` frontmatter override still works (intersection with role tools)
- [x] Remove `isSkillMatchingRole()`, `addAllowedTool()`, `addToolsForCategory()`, `addSchemaTools()` functions
- [x] Remove `allowedToolSetFromSkills()` function
- [x] `FilterToolsBySkills()` → simplified or removed
- [x] All existing prompt assembly tests pass
- [x] Unit test: backend role gets edit tools; reviewer does not
- [x] Unit test: JIT override further restricts tool set

### Task 3.4: Wire Registry into Orchestrator ✅
> Links to: REQ-002

**Acceptance Criteria:**
- [x] `Orchestrator` creates `tool.Registry` at initialization
- [x] All built-in tools registered via `DefaultRegistry()` function
- [x] `CapabilityManager` instantiated with `DefaultRoleProfiles()`
- [x] `llm_step.go` passes registry to `llmrunner.Runner`
- [x] `step_registry.go` passes `CapabilityManager` to prompt assembler
- [x] Existing orchestrator tests pass

### Task 3.5: Update Prompt Tool Descriptions for LLM ✅
> Links to: REQ-013

**Acceptance Criteria:**
- [x] System prompt includes a `## Available Tools` section listing tools by category
- [x] Each tool entry includes: name, description, parameter list, and a concise usage example
- [x] Tool descriptions section tracked in `BudgetTrace.ToolTokens`
- [x] Section is generated dynamically from registry definitions (not hardcoded)
- [x] Unit test: prompt with backend role includes edit tool descriptions

---

## P3 — Low (Advanced Tools)

### Task 4.1: Implement `find_symbol` Tool
> Links to: REQ-007

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/find_symbol.go`
- [x] Uses AST/ctags to find symbol definitions matching a name
- [x] Returns file, line, and signature
- [x] Category: `search`, Capabilities: `[CapSearch]`

### Task 4.2: Implement `run_lint` Tool
> Links to: REQ-010

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/run_lint.go`
- [x] Runs configured linter (e.g., `golangci-lint run`) in sandbox
- [x] Parses lint output into `Diagnostics`
- [x] Category: `build`, Capabilities: `[CapBuild]`

### Task 4.3: Implement `git_checkpoint` and `git_restore` Tools
> Links to: REQ-008

**Acceptance Criteria:**
- [x] `git_checkpoint`: commits current state with checkpoint message, returns commit hash
- [x] `git_restore`: restores to a given commit hash via `git checkout + reset`
- [x] Both operate within the task's worktree
- [x] Category: `git`, Capabilities: `[CapGit]`

### Task 4.4: Implement `create_file` Tool (Gated write_file)
> Links to: REQ-005

**Acceptance Criteria:**
- [x] Create `server/internal/tool/tools/create_file.go`
- [x] Only creates new files or appends to empty files — refuses to overwrite existing content
- [x] Category: `filesystem`, Capabilities: `[CapCreate]`
- [x] Not included in default role profiles (must be explicitly granted)

### Task 4.5: Remove Legacy Code
> Links to: REQ-R01, REQ-R02, REQ-R03, REQ-R04

**Acceptance Criteria:**
- [x] Delete `BuiltinToolDefinitions()` from `skills/tools.go`
- [x] Delete `SkillExecutor.Execute()` switch and all private tool methods from `skills/executor.go`
- [x] Delete `isSkillMatchingRole()`, `addAllowedTool()`, `addToolsForCategory()`, `addSchemaTools()`, `allowedToolSetFromSkills()` from `prompts/tools.go`
- [x] Delete `analyzeToolDefinitions()` and `executeAnalyzeTool()` from `steps/analyze_tools.go`
- [x] Verify no imports reference deleted functions
- [x] All tests pass after cleanup

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
