# Proposal: Tool Runtime Refactor — From Utility Functions to Capability-Based Agent Tooling

## Why

The current tool system in `auto_code_os` was designed as a collection of **hardcoded utility functions** — an appropriate MVP, but fundamentally misaligned with the architecture of production-grade autonomous coding agents (Claude Code, OpenAI Codex CLI, Gemini CLI).

### Root Cause Analysis

1. **Monolithic Switch Dispatch**: `SkillExecutor.Execute()` is a giant `switch` statement that maps tool names to private methods. Adding a new tool requires modifying 3 files (definition, executor, filter) and re-deploying the entire binary.

2. **Coupled Definition & Implementation**: `BuiltinToolDefinitions()` returns `[]llm.ToolDefinition` as a flat slice of hardcoded JSON schemas. The schema lives in one package (`skills/tools.go`), execution in another (`skills/executor.go`), and filtering logic in a third (`prompts/tools.go`). There is no interface contract linking a tool's schema to its implementation.

3. **No Edit Safety**: `write_file` allows the LLM to overwrite entire files with no verification, diffing, checkpointing, or rollback. This is the single highest-risk surface in the system. Modern agents (Claude Code, Codex) use `search_replace` / `edit_file` with mandatory pre/post verification.

4. **Primitive `SkillResult`**: The result type is `{Name, Output, Success, Error}` — a string-only contract. There is no structured metadata (files changed, line ranges, hashes, diagnostics) that downstream pipeline stages can consume programmatically.

5. **Flat Tool Exposure**: All 8 tools are treated as peers. There is no capability grouping, no role-based subset selection at registration time, and the filtering logic in `prompts/tools.go` is a fragile web of string-matching heuristics.

6. **Underpowered Read/Search**: `read_file` truncates at `max_bytes` (no line-range support), and `search_code` only does `strings.Contains()` — no regex, no symbol resolution, no line-number output.

7. **Missing Critical Tools**: No `git_diff`, `git_status`, `git_checkpoint`, `run_build`, `run_lint`, `find_symbol`, `find_definition`, `list_files` (in builtin set), `file_exists`.

---

## What Changes

### Issue 1: Monolithic Tool Registry — Decouple Definition from Implementation

- Introduce a `tool.Tool` interface with `Name()`, `Description()`, `Schema()`, `Execute()` methods.
- Create a `tool.Registry` that holds registered `Tool` implementations.
- Each tool becomes a self-contained struct in its own file (e.g., `tools/read_file.go`).
- Delete the switch-based `SkillExecutor.Execute()` dispatch. Registry handles routing.

### Issue 2: No Capability Grouping — Introduce Capability Categories

- Define tool categories: `Filesystem`, `Git`, `Search`, `Build`, `Editing`, `Context`, `Documentation`.
- Each `Tool` declares its `Category()`.
- `CapabilityManager` exposes category → tool-set mappings.
- Agent roles receive tool subsets via declarative capability profiles (not runtime string-matching).

### Issue 3: Unsafe Writes — Replace `write_file` with Transactional Editing

- Remove `write_file` from default agent toolsets.
- Promote `search_replace` (renamed from `apply_patch`) as the primary editing primitive.
- Add `edit_file` tool that wraps a search/replace transaction with:
  - `DryRun` mode (returns diff preview without applying)
  - Automatic `go fmt` / prettier post-hook
  - Pre/post content hash verification
  - Compile-check verification hook
- `write_file` retained only in `Filesystem` category, gated behind `create_file` capability.

### Issue 4: Primitive ToolResult — Standardize Result Contract

- Replace `SkillResult` with `tool.Result`:
  ```go
  type Result struct {
      Success      bool
      Message      string
      FilesChanged []string
      Diagnostics  []Diagnostic
      Metadata     map[string]any
  }
  ```
- All tools return structured metadata (line ranges, hashes, match counts, etc.).

### Issue 5: Flat Tool Exposure — Capability-Based Role Profiles

- Define role → capability profiles declaratively:
  | Role     | Capabilities                         |
  |----------|--------------------------------------|
  | Planner  | Read, Search, Context, Documentation |
  | Backend  | Read, Edit, Build, Git, Search       |
  | Frontend | Read, Edit, Build, Search            |
  | Reviewer | Read, Search, Git (diff only)        |
  | Security | Read, Search, Dependency             |
- Replace the `isSkillMatchingRole()` + `FilterToolsBySkills()` + `addToolsForCategory()` heuristic chain with a single `registry.ToolsForCapabilities(caps)` call.

### Issue 6: Underpowered Read/Search — Upgrade Core Tools

- `read_file`: Add `start_line`, `end_line`, `around_line`, `radius` parameters.
- `search_code` → `grep_search`: Add `regex`, `include`, `exclude`, line-number output.
- Add `find_symbol`, `find_definition`, `find_references` tools backed by AST/ctags.

### Issue 7: Missing Tools — Add Critical Tool Set

- **Git**: `git_diff`, `git_status`, `git_checkpoint`, `git_restore`
- **Workspace**: `list_files`, `tree`, `file_exists`
- **Build**: `run_build`, `run_lint`, `run_tests`, `run_single_test`
- **Editing**: `insert_lines`, `replace_lines`
- **Context**: `read_spec`, `read_architecture`, `read_affected_files`

### Issue 8: No Verification Pipeline — Add Post-Edit Verification

- After any edit tool, run a configurable verification pipeline:
  `edit → format → compile-check → lint → success/rollback`
- Verification hooks are registered per-language in the tool registry.

---

## Capabilities

### New Capabilities
- `tool.Tool` interface and `tool.Registry` with plugin-style registration
- `tool.CapabilityManager` for role → tool-set resolution
- `tool.Result` standardized result type with structured metadata
- `edit_file` / `search_replace` transactional editing tools
- `read_file` line-range and around-line support
- `grep_search` with regex, includes/excludes, line numbers
- `find_symbol` / `find_definition` AST-backed search
- Git tools: `git_diff`, `git_status`, `git_checkpoint`, `git_restore`
- Workspace tools: `list_files`, `tree`, `file_exists`
- Build tools: `run_build`, `run_lint`, `run_single_test`
- Context tools: `read_spec`, `read_architecture`
- Post-edit verification pipeline
- Declarative role → capability profiles

### Modified Capabilities
- `read_file` → enhanced with line-range parameters
- `apply_patch` → renamed to `search_replace`, enhanced with dry-run and verification
- `search_code` → renamed to `grep_search`, enhanced with regex and line output
- `run_tests` → enhanced with structured test result parsing
- `SkillResult` → replaced by `tool.Result`
- `BuiltinToolDefinitions()` → replaced by `Registry.Definitions()`
- `SkillExecutor.Execute()` → replaced by `Registry.Execute()`
- `FilterToolsBySkills()` → replaced by `Registry.ToolsForCapabilities()`

### Removed Capabilities
- `SkillExecutor` monolithic switch dispatch
- `BuiltinToolDefinitions()` hardcoded slice
- `isSkillMatchingRole()` / `addToolsForCategory()` heuristic chain
- `write_file` as default tool (retained, but gated behind `create_file` capability)

---

## Impact

| Area | Files Affected |
|------|----------------|
| **Tool Runtime (new)** | `server/internal/tool/tool.go` (interface) |
| | `server/internal/tool/registry.go` (registry) |
| | `server/internal/tool/result.go` (result type) |
| | `server/internal/tool/capability.go` (capability manager) |
| | `server/internal/tool/verify.go` (verification pipeline) |
| **Tool Implementations (new)** | `server/internal/tool/tools/read_file.go` |
| | `server/internal/tool/tools/search_replace.go` |
| | `server/internal/tool/tools/grep_search.go` |
| | `server/internal/tool/tools/list_files.go` |
| | `server/internal/tool/tools/git_diff.go` |
| | `server/internal/tool/tools/git_status.go` |
| | `server/internal/tool/tools/run_tests.go` |
| | `server/internal/tool/tools/run_build.go` |
| | `server/internal/tool/tools/find_symbol.go` |
| | `server/internal/tool/tools/read_spec.go` |
| | *(~20 tool files total)* |
| **Skills (deprecate)** | `server/internal/orchestrator/skills/tools.go` → adapter |
| | `server/internal/orchestrator/skills/executor.go` → adapter |
| | `server/internal/orchestrator/skills/sandbox.go` → adapter |
| **Prompt Assembly** | `server/internal/prompts/tools.go` → simplified |
| | `server/internal/prompts/builder.go` → uses registry |
| **LLM Runner** | `server/internal/orchestrator/llmrunner/runner.go` → uses tool result |
| **Step Files** | `server/internal/orchestrator/steps/analyze_tools.go` → uses registry |
| **Orchestrator** | `server/internal/orchestrator/llm_step.go` → wires registry |
| | `server/internal/orchestrator/step_registry.go` → wires registry |
| **LLM Types** | `server/pkg/llm/provider.go` → `ToolDefinition` unchanged |
