# Context Engine Integration: Context Load Pre-warming — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Integrate the Phase 13-15 `ContextEngine` into the orchestrator's `ContextLoad` step. Currently, the `ContextLoad` step collects static file context (git logs, branches, etc.) but does not actively engage the new SQLite AST Context Engine. To avoid synchronous blocking and timeouts during the first LLM prompt assembly (which calls `GetRepoMap`), we must pre-warm/index the AST cache during the dedicated `ContextLoad` step.

**Architecture:**
1. Expose a public `IndexWorkspace(ctx context.Context)` method on the `ContextEngine` interface.
2. Inject the `ContextEngine` into `ContextLoadStep` (via `NewContextLoadStep`).
3. Call `IndexWorkspace` during `ContextLoadStep.Execute()` to prepopulate the SQLite AST cache.
4. Clean up legacy fields/logic in `context_load.go` that are no longer needed after the removal of LLM-based profile generation.

**Tech Stack:** Go 1.23.

---

## 1. Context Engine Interface Update
- [x] **1.1** In `server/internal/context/provider/provider.go`, add `IndexWorkspace(ctx context.Context) error` to the `ContextEngine` interface.
- [x] **1.2** Rename the existing unexported `ensureCachePopulated` method to `IndexWorkspace(ctx context.Context) error` and ensure it fulfills the new interface requirement.

## 2. Dependency Injection for ContextLoadStep
- [x] **2.1** In `server/internal/orchestrator/steps/context_load.go`, add `ctxEngine provider.ContextEngine` to the `ContextLoadStep` struct.
- [x] **2.2** Update the constructor `NewContextLoadStep` to accept `ctxEngine provider.ContextEngine` as an argument.
- [x] **2.3** Update `server/internal/orchestrator/orchestrator.go` (where the steps are wired up) to pass `o.ctxEngine` into `NewContextLoadStep`.

## 3. Pre-Warming Execution
- [x] **3.1** In `ContextLoadStep.Execute()`, after resolving repository paths, invoke `s.ctxEngine.IndexWorkspace(ctx)`. This leverages the dedicated Context Loading workflow phase to do the heavy I/O of parsing and caching ASTs in SQLite.
- [x] **3.2** Handle errors gracefully (e.g., log a warning if indexing fails, but do not necessarily fail the entire step unless critical).

## 4. Test Updates
- [x] **4.1** In `server/internal/orchestrator/steps/context_load_test.go`, update the `buildStep` initialization to pass a mock `ContextEngine`.
- [x] **4.2** Add a mock implementation of `ContextEngine` in the test file that records calls to `IndexWorkspace`.
- [x] **4.3** Assert that `IndexWorkspace` is called exactly once during the `ContextLoadStep.Execute()` test.

## 5. Cleanups (Optional/Recommended)
- [x] **5.1** Remove `LLMChatter` from `ContextLoadStep` if it's no longer used. Since LLM-based architecture profiling was removed from `context_load.go`, the `llm` dependency is now dead code inside `ContextLoadStep`.
- [x] **5.2** Update the step factory/wiring to stop passing the LLM provider to `NewContextLoadStep`.

## 6. Role-based Prompt Architecture Refactoring (Prompt Engineering)
- [x] **6.1** **Decouple Prompt Core**: Extract the core instructions of system prompts into separate `.md` files (e.g., `prompts/roles/planner.md`, `prompts/roles/coder.md`, `prompts/roles/reviewer.md`) to enable easy version control and readability without recompiling the source code (Reference: `ai-sdlc.md` Item 14).
- [x] **6.2** **Dynamic Metadata Injection (`appendSystemPrompt`)**: Implement an `appendSystemPrompt` function (or equivalent in the `PromptAssembler`) to dynamically inject JSON/YAML configuration and metadata at the end of the core Markdown prompts.
- [x] **6.3** **Role-Specific Context Optimization**: Tailor the context injected based on the AI role to avoid token bloat and hallucination:
  - **Coder/Reviewer:** Inject the Aider-style "Repo Map" (AST skeleton) to provide a super-compressed dependency graph without the function bodies (Reference: `aider.md`).
  - **Planner:** Provide high-level architecture documents and task dependency DAGs.
- [x] **6.4** **Clean Up Hardcoded Prompts**: Remove legacy hardcoded, monolithic system prompt strings from the Go source files (`server/internal/orchestrator/prompts/...`) and replace them with dynamic file reads from the `prompts/` directory.
- [x] **6.5** **Completely Remove Legacy Fallback**: Remove `personaFile` and fallback logic to `Prompt Base`, fully isolating the orchestrator's role logic to the internal `prompts/roles/*.md` definitions.
