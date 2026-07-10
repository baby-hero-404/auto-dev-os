# Specs: Tool Runtime Refactor

## Added Requirements

### REQ-001: Tool Interface Contract
> ❌ Status: Not Started

**Scenario:**
- WHEN a developer creates a new tool struct implementing `tool.Tool`
- THEN it must satisfy `Name()`, `Description()`, `Schema()`, `Category()`, and `Execute()` methods
- AND the compiler enforces the interface at build time
- AND no changes to the registry dispatcher or switch statements are required

### REQ-002: Tool Registry with Self-Registration
> ❌ Status: Not Started

**Scenario:**
- WHEN the application starts
- THEN all built-in tools are registered via `registry.Register(tool)` in an `init()` or explicit wire-up
- AND `registry.Definitions()` returns `[]llm.ToolDefinition` for all registered tools
- AND `registry.Execute(ctx, name, call)` dispatches to the correct tool implementation
- AND calling `registry.Execute()` with an unknown tool name returns a structured error

### REQ-003: Capability-Based Tool Exposure
> ❌ Status: Not Started

**Scenario:**
- WHEN the prompt assembler resolves tools for a Backend agent
- THEN only tools matching capabilities `[Read, Edit, Build, Git, Search]` are returned
- AND `write_file` (capability: `Create`) is NOT included unless explicitly granted
- AND a Reviewer agent receives only `[Read, Search, Git.Diff]` tools

**Scenario (role profile):**
- WHEN a new role `"security-auditor"` is defined with capabilities `[Read, Search, Dependency]`
- THEN `registry.ToolsForRole("security-auditor")` returns exactly the tools in those categories
- AND no code changes to `isSkillMatchingRole()` or `addToolsForCategory()` are required

### REQ-004: Standardized Tool Result
> ❌ Status: Not Started

**Scenario:**
- WHEN `search_replace` executes successfully
- THEN the result contains `Success: true`, `FilesChanged: ["path/to/file.go"]`, `Metadata: {"replaced_count": 1, "hash_before": "abc", "hash_after": "def"}`
- AND downstream steps can programmatically inspect `result.FilesChanged` without string parsing

**Scenario (failure):**
- WHEN `read_file` is called with a path that escapes the workspace
- THEN the result contains `Success: false`, `Diagnostics: [{Severity: "error", Message: "path escapes workspace"}]`
- AND `FilesChanged` is empty

### REQ-005: Transactional Edit Tool (search_replace)
> ❌ Status: Not Started

**Scenario (normal edit):**
- WHEN the LLM calls `search_replace` with `{path, search, replace}`
- THEN the tool verifies the search block exists exactly once in the file
- AND applies the replacement
- AND runs the configured format hook (e.g. `gofmt`)
- AND returns `hash_before`, `hash_after`, and the replaced line range

**Scenario (dry-run):**
- WHEN the LLM calls `search_replace` with `{path, search, replace, dry_run: true}`
- THEN the tool returns the diff preview WITHOUT modifying the file
- AND `FilesChanged` is empty

**Scenario (search not found):**
- WHEN the search block does not exist in the target file
- THEN the tool returns `Success: false` with diagnostic `"search block not found in file"`
- AND the file is not modified

### REQ-006: Enhanced read_file with Line Ranges
> ❌ Status: Not Started

**Scenario (full file):**
- WHEN the LLM calls `read_file` with `{path: "main.go"}`
- THEN the entire file content is returned (up to max_lines default 500)

**Scenario (line range):**
- WHEN the LLM calls `read_file` with `{path: "main.go", start_line: 50, end_line: 80}`
- THEN only lines 50-80 are returned
- AND the result metadata includes `total_lines` of the file

**Scenario (around line):**
- WHEN the LLM calls `read_file` with `{path: "main.go", around_line: 150, radius: 30}`
- THEN lines 120-180 are returned
- AND context is preserved for the LLM to make precise edits

### REQ-007: Enhanced grep_search with Regex and Line Numbers
> ❌ Status: Not Started

**Scenario (literal):**
- WHEN the LLM calls `grep_search` with `{query: "func Execute"}`
- THEN matching lines are returned with file path and line number: `"executor.go:36: func (e *SkillExecutor) Execute(...)"`

**Scenario (regex):**
- WHEN the LLM calls `grep_search` with `{query: "func\\s+\\(.*\\)\\s+Execute", regex: true}`
- THEN regex pattern matching is applied
- AND results include line numbers

**Scenario (filtered):**
- WHEN the LLM calls `grep_search` with `{query: "TODO", include: "*.go", exclude: "vendor/*"}`
- THEN only `.go` files outside `vendor/` are searched

### REQ-008: Git Tools
> ❌ Status: Not Started

**Scenario (git_diff):**
- WHEN the LLM calls `git_diff` with `{}`
- THEN the unstaged diff of the worktree is returned
- AND the result includes `FilesChanged` listing modified files

**Scenario (git_status):**
- WHEN the LLM calls `git_status`
- THEN the output shows staged, unstaged, and untracked files in structured format

### REQ-009: Workspace Tools
> ❌ Status: Not Started

**Scenario (list_files):**
- WHEN the LLM calls `list_files` with `{path: "internal/", max_depth: 2}`
- THEN a tree of files/directories is returned, pruning `.git`, `node_modules`, `vendor`

**Scenario (file_exists):**
- WHEN the LLM calls `file_exists` with `{path: "go.mod"}`
- THEN `Success: true` is returned with metadata `{exists: true, size: 1234}`

### REQ-010: Build & Validation Tools
> ❌ Status: Not Started

**Scenario (run_build):**
- WHEN the LLM calls `run_build`
- THEN `go build ./...` (or configured command) is executed in sandbox
- AND structured build errors are returned in `Diagnostics`

**Scenario (run_lint):**
- WHEN the LLM calls `run_lint`
- THEN the configured linter runs and structured lint results are returned

### REQ-011: Post-Edit Verification Pipeline
> ❌ Status: Not Started

**Scenario:**
- WHEN `search_replace` completes successfully
- THEN the verification pipeline runs: `format → compile-check`
- AND if compile-check fails, the edit is auto-rolled-back
- AND the tool result includes `Diagnostics` from the failed verification step
- AND `Success` is set to `false`

**Scenario (verification disabled):**
- WHEN `search_replace` is called with `{verify: false}`
- THEN the verification pipeline is skipped
- AND the raw edit result is returned

### REQ-012: Context-Aware Tools
> ❌ Status: Not Started

**Scenario:**
- WHEN the LLM calls `read_spec`
- THEN the tool resolves the current task's OpenSpec files from `docs/openspecs/` and returns their contents
- AND the LLM does not need to know the file path

**Scenario:**
- WHEN the LLM calls `read_affected_files`
- THEN the tool reads `task.Analysis.AffectedFiles` and returns their contents
- AND worktree-first resolution is applied automatically

### REQ-013: Prompt Integration — Tool Descriptions in System Prompt
> ❌ Status: Not Started

**Scenario:**
- WHEN the prompt assembler builds the system message for a coding step
- THEN tool descriptions are injected as a structured section listing available tools, their parameters, and usage examples
- AND the section is ordered by tool category
- AND the token cost of tool descriptions is tracked in `BudgetTrace`

---

## Modified Requirements

### REQ-M01: Backward-Compatible SkillExecutor Adapter
> ❌ Status: Not Started

**Scenario:**
- WHEN legacy code calls `SkillExecutor.Execute(ctx, call)`
- THEN the adapter delegates to `registry.Execute(ctx, call.Name, call)`
- AND the `tool.Result` is converted back to `SkillResult` for compatibility
- AND no existing step code breaks during the migration

### REQ-M02: Analyze Step Tool Migration
> ❌ Status: Not Started

**Scenario:**
- WHEN the analyze step requests tool definitions via `analyzeToolDefinitions()`
- THEN it receives definitions from the shared registry filtered by `[Read, Search]` capabilities
- AND the `list_files`, `read_file`, `grep_search` tools are identical to the ones used in coding steps
- AND the `executeAnalyzeTool()` switch is replaced by `registry.Execute()`

### REQ-M03: Prompt Tool Assembly Simplification
> ❌ Status: Not Started

**Scenario:**
- WHEN `toolDefinitionsForAgent()` is called
- THEN it resolves the agent's role → capability profile
- AND calls `registry.ToolsForCapabilities(caps)` to get `[]llm.ToolDefinition`
- AND the entire `allowedToolSetFromSkills()` / `addToolsForCategory()` / `isSkillMatchingRole()` chain is removed
- AND JIT skill `allowed-tools` frontmatter overrides are still respected

---

## Removed Requirements

- REQ-R01: `BuiltinToolDefinitions()` flat slice — replaced by `Registry.Definitions()`
- REQ-R02: `SkillExecutor.Execute()` switch dispatch — replaced by `Registry.Execute()`
- REQ-R03: `addAllowedTool()` / `addToolsForCategory()` heuristic functions — replaced by capability profiles
- REQ-R04: `write_file` as default tool for all agents — gated behind `Create` capability
