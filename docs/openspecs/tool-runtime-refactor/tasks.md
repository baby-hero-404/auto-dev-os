# Tasks: Tool Runtime Refactor

## P0 — Critical (Foundation)

### Task 1.1: Define Core Tool Interface & Types
> Links to: REQ-001, REQ-004

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tool.go` with `Tool` interface: `Name()`, `Description()`, `Schema()`, `Category()`, `Capabilities()`, `Execute()`
- [ ] Define `Category` constants: `filesystem`, `editing`, `git`, `search`, `build`, `context`, `documentation`
- [ ] Define `Capability` constants: `read`, `edit`, `create`, `search`, `build`, `git`, `git.diff`, `context`, `docs`, `dependency`
- [ ] Define `Call` struct: `Input`, `Workspace`, `TaskID`, `AgentID`, `AgentRole`
- [ ] Define `Result` struct: `Success`, `Message`, `Output`, `FilesChanged`, `Diagnostics`, `Metadata`
- [ ] Define `Diagnostic` struct: `Severity`, `File`, `Line`, `Message`
- [ ] Unit test: verify `Result` JSON marshaling with all fields populated

### Task 1.2: Implement Tool Registry
> Links to: REQ-002

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/registry.go` with `Registry` struct
- [ ] `Register(tool)` adds tool to map; panics on duplicate name
- [ ] `Execute(ctx, name, call)` dispatches to correct tool; returns error for unknown tool
- [ ] `Definitions()` returns `[]llm.ToolDefinition` for all registered tools
- [ ] `ToolsForCapabilities(caps)` returns filtered definitions matching any provided capability
- [ ] Thread-safe via `sync.RWMutex`
- [ ] Unit test: register 3 tools → verify `Definitions()` returns 3 items
- [ ] Unit test: `Execute()` with unknown name → returns structured error
- [ ] Unit test: `ToolsForCapabilities([CapRead])` → returns only read-capable tools

### Task 1.3: Implement Capability Manager & Role Profiles
> Links to: REQ-003

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/capability.go` with `CapabilityManager` and `RoleProfile`
- [ ] `DefaultRoleProfiles()` returns profiles for: planner, backend, frontend, reviewer, qa, security-auditor
- [ ] `ToolsForRole(role)` resolves role → capabilities → filtered tool definitions
- [ ] Unknown role falls back to `[CapRead, CapSearch]`
- [ ] Unit test: `ToolsForRole("backend")` includes edit tools, excludes docs-only tools
- [ ] Unit test: `ToolsForRole("reviewer")` excludes edit tools
- [ ] Unit test: `ToolsForRole("unknown_role")` returns read + search tools only

### Task 1.4: Implement `read_file` Tool (Enhanced)
> Links to: REQ-006

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/read_file.go` implementing `tool.Tool`
- [ ] Parameters: `path` (required), `start_line`, `end_line`, `around_line`, `radius`, `max_lines` (default 500)
- [ ] Full file read: returns entire content up to `max_lines`
- [ ] Line range: `start_line=50, end_line=80` returns only those lines
- [ ] Around line: `around_line=150, radius=30` returns lines 120-180
- [ ] Result metadata includes `total_lines`, `returned_lines`, `start_line`, `end_line`
- [ ] Path validation via `SafeWorkspacePath()` — rejects directory traversal
- [ ] Category: `filesystem`, Capabilities: `[CapRead]`
- [ ] Unit test: full file read with max_lines truncation
- [ ] Unit test: line range read
- [ ] Unit test: around_line read
- [ ] Unit test: path traversal rejection

### Task 1.5: Implement `search_replace` Tool (Transactional)
> Links to: REQ-005, REQ-011

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/search_replace.go` implementing `tool.Tool`
- [ ] Parameters: `path`, `search`, `replace` (required); `dry_run`, `verify` (optional, default false/true)
- [ ] Validates search block exists exactly once in file
- [ ] Returns `hash_before`, `hash_after`, `replaced_count` in metadata
- [ ] Dry-run mode: returns diff preview without modifying file
- [ ] Verification pipeline: runs format hook → compile check after successful edit
- [ ] Rollback: if verification fails, restores original file content
- [ ] Result includes `FilesChanged` on success
- [ ] Category: `editing`, Capabilities: `[CapEdit]`
- [ ] Unit test: successful search/replace with hash verification
- [ ] Unit test: search block not found → error, file unchanged
- [ ] Unit test: ambiguous match (2 occurrences) → error, file unchanged
- [ ] Unit test: dry-run returns preview without modification
- [ ] Unit test: verification failure triggers rollback

### Task 1.6: Implement `grep_search` Tool (Enhanced)
> Links to: REQ-007

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/grep_search.go` implementing `tool.Tool`
- [ ] Parameters: `query` (required), `regex` (bool), `include` (glob), `exclude` (glob), `max_results` (default 30)
- [ ] Literal mode: uses `strings.Contains` per-line with line numbers
- [ ] Regex mode: uses `regexp.Compile` with match extraction
- [ ] Output format: `filepath:line_number: matching line content`
- [ ] Respects include/exclude globs; always skips `.git`, `node_modules`, `vendor`
- [ ] Category: `search`, Capabilities: `[CapSearch]`
- [ ] Unit test: literal search returns line numbers
- [ ] Unit test: regex search with capture
- [ ] Unit test: include/exclude glob filtering

---

## P1 — High (Core Tools)

### Task 2.1: Implement `list_files` Tool
> Links to: REQ-009

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/list_files.go`
- [ ] Parameters: `path` (default "."), `max_depth` (default 3), `max_files` (default 200)
- [ ] Returns tree structure as formatted text
- [ ] Skips `.git`, `node_modules`, `vendor`, `dist`, `build`
- [ ] Category: `filesystem`, Capabilities: `[CapRead]`
- [ ] Unit test: tree output at depth 2

### Task 2.2: Implement `file_exists` Tool
> Links to: REQ-009

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/file_exists.go`
- [ ] Parameters: `path` (required)
- [ ] Returns `{exists: bool, size: int, is_dir: bool}` in metadata
- [ ] Category: `filesystem`, Capabilities: `[CapRead]`
- [ ] Unit test: existing file → true; missing file → false

### Task 2.3: Implement `git_diff` Tool
> Links to: REQ-008

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/git_diff.go`
- [ ] Parameters: `staged` (bool, default false), `path` (optional filter)
- [ ] Runs `git diff` (or `git diff --staged`) in sandbox
- [ ] Returns diff output with `FilesChanged` list
- [ ] Category: `git`, Capabilities: `[CapGit, CapGitDiff]`
- [ ] Unit test: mock sandbox returns diff → parsed FilesChanged

### Task 2.4: Implement `git_status` Tool
> Links to: REQ-008

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/git_status.go`
- [ ] Runs `git status --porcelain` in sandbox
- [ ] Parses output into structured `{staged: [], unstaged: [], untracked: []}` metadata
- [ ] Category: `git`, Capabilities: `[CapGit]`
- [ ] Unit test: parse porcelain output

### Task 2.5: Implement `run_tests` Tool (Enhanced)
> Links to: REQ-010

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/run_tests.go`
- [ ] Parameters: `command` (default "go test ./..."), `path` (optional target)
- [ ] Runs command in sandbox
- [ ] Parses test output into `Diagnostics` (pass/fail per test)
- [ ] Category: `build`, Capabilities: `[CapBuild]`
- [ ] Unit test: parse Go test output with failures

### Task 2.6: Implement `run_build` Tool
> Links to: REQ-010

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/run_build.go`
- [ ] Parameters: `command` (default "go build ./...")
- [ ] Runs in sandbox; parses compiler errors into `Diagnostics` with file + line
- [ ] Category: `build`, Capabilities: `[CapBuild]`
- [ ] Unit test: parse Go compiler error output

### Task 2.7: Implement Verification Pipeline
> Links to: REQ-011

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/verify.go` with `VerifyHook` interface and `VerifyPipeline`
- [ ] Create `server/internal/tool/verify/gofmt.go` — runs `gofmt -w` on changed `.go` files
- [ ] Create `server/internal/tool/verify/compile_check.go` — runs `go build ./...` and parses errors
- [ ] Pipeline stops on first error-severity diagnostic
- [ ] Unit test: gofmt hook formats file; compile_check detects syntax error

### Task 2.8: Implement Context Tools
> Links to: REQ-012

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/read_spec.go` — resolves `docs/openspecs/<task>/` and returns spec contents
- [ ] Create `server/internal/tool/tools/read_affected_files.go` — reads from `task.Analysis.AffectedFiles` with worktree-first resolution
- [ ] Category: `context`, Capabilities: `[CapContext]`
- [ ] Unit test: read_spec resolves correct files from analysis

---

## P2 — Medium (Integration & Migration)

### Task 3.1: Backward-Compatible SkillExecutor Adapter
> Links to: REQ-M01

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/adapter.go`
- [ ] `SkillExecutorAdapter` wraps `tool.Registry` and implements the old `SkillExecutor.Execute(ctx, SkillCall) SkillResult` signature
- [ ] Converts `SkillCall` → `tool.Call`, dispatches to registry, converts `tool.Result` → `SkillResult`
- [ ] Existing step code (`code_backend.go`, `code_frontend.go`, `fix.go`) works without modification
- [ ] Unit test: adapter converts call and result correctly
- [ ] Unit test: adapter handles unknown tool gracefully

### Task 3.2: Migrate Analyze Step Tools
> Links to: REQ-M02

**Acceptance Criteria:**
- [ ] `analyzeToolDefinitions()` in `steps/analyze_tools.go` → returns `registry.ToolsForCapabilities([CapRead, CapSearch])`
- [ ] `executeAnalyzeTool()` switch → replaced by `registry.Execute(ctx, toolName, call)`
- [ ] `listAnalyzeFiles()`, `readAnalyzeFile()`, `grepAnalyzeFiles()` logic moved to `list_files`, `read_file`, `grep_search` tools
- [ ] All existing analyze tests pass without modification
- [ ] Unit test: analyze step uses registry tools

### Task 3.3: Simplify Prompt Tool Assembly
> Links to: REQ-M03

**Acceptance Criteria:**
- [ ] `toolDefinitionsForAgent()` in `prompts/tools.go` → resolves role via `CapabilityManager.ToolsForRole(agent.Role)`
- [ ] JIT skill `allowed-tools` frontmatter override still works (intersection with role tools)
- [ ] Remove `isSkillMatchingRole()`, `addAllowedTool()`, `addToolsForCategory()`, `addSchemaTools()` functions
- [ ] Remove `allowedToolSetFromSkills()` function
- [ ] `FilterToolsBySkills()` → simplified or removed
- [ ] All existing prompt assembly tests pass
- [ ] Unit test: backend role gets edit tools; reviewer does not
- [ ] Unit test: JIT override further restricts tool set

### Task 3.4: Wire Registry into Orchestrator
> Links to: REQ-002

**Acceptance Criteria:**
- [ ] `Orchestrator` creates `tool.Registry` at initialization
- [ ] All built-in tools registered via `DefaultRegistry()` function
- [ ] `CapabilityManager` instantiated with `DefaultRoleProfiles()`
- [ ] `llm_step.go` passes registry to `llmrunner.Runner`
- [ ] `step_registry.go` passes `CapabilityManager` to prompt assembler
- [ ] Existing orchestrator tests pass

### Task 3.5: Update Prompt Tool Descriptions for LLM
> Links to: REQ-013

**Acceptance Criteria:**
- [ ] System prompt includes a `## Available Tools` section listing tools by category
- [ ] Each tool entry includes: name, description, parameter list, and a concise usage example
- [ ] Tool descriptions section tracked in `BudgetTrace.ToolTokens`
- [ ] Section is generated dynamically from registry definitions (not hardcoded)
- [ ] Unit test: prompt with backend role includes edit tool descriptions

---

## P3 — Low (Advanced Tools)

### Task 4.1: Implement `find_symbol` Tool
> Links to: REQ-007

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/find_symbol.go`
- [ ] Uses AST/ctags to find symbol definitions matching a name
- [ ] Returns file, line, and signature
- [ ] Category: `search`, Capabilities: `[CapSearch]`

### Task 4.2: Implement `run_lint` Tool
> Links to: REQ-010

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/run_lint.go`
- [ ] Runs configured linter (e.g., `golangci-lint run`) in sandbox
- [ ] Parses lint output into `Diagnostics`
- [ ] Category: `build`, Capabilities: `[CapBuild]`

### Task 4.3: Implement `git_checkpoint` and `git_restore` Tools
> Links to: REQ-008

**Acceptance Criteria:**
- [ ] `git_checkpoint`: commits current state with checkpoint message, returns commit hash
- [ ] `git_restore`: restores to a given commit hash via `git checkout + reset`
- [ ] Both operate within the task's worktree
- [ ] Category: `git`, Capabilities: `[CapGit]`

### Task 4.4: Implement `create_file` Tool (Gated write_file)
> Links to: REQ-005

**Acceptance Criteria:**
- [ ] Create `server/internal/tool/tools/create_file.go`
- [ ] Only creates new files or appends to empty files — refuses to overwrite existing content
- [ ] Category: `filesystem`, Capabilities: `[CapCreate]`
- [ ] Not included in default role profiles (must be explicitly granted)

### Task 4.5: Remove Legacy Code
> Links to: REQ-R01, REQ-R02, REQ-R03, REQ-R04

**Acceptance Criteria:**
- [ ] Delete `BuiltinToolDefinitions()` from `skills/tools.go`
- [ ] Delete `SkillExecutor.Execute()` switch and all private tool methods from `skills/executor.go`
- [ ] Delete `isSkillMatchingRole()`, `addAllowedTool()`, `addToolsForCategory()`, `addSchemaTools()`, `allowedToolSetFromSkills()` from `prompts/tools.go`
- [ ] Delete `analyzeToolDefinitions()` and `executeAnalyzeTool()` from `steps/analyze_tools.go`
- [ ] Verify no imports reference deleted functions
- [ ] All tests pass after cleanup
