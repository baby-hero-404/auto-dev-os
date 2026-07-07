# Context Engine Integration — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Inject the `ContextEngine` (developed in Phase 13) into the orchestrator's `PromptAssembler`. This ensures that every LLM request receives a lightweight, token-pruned Repository Map (Skeleton Map) alongside the standard Vector Snippets, granting the AI complete structural awareness without hallucination.

**Architecture:** 
Modify `PromptAssembler` to hold an instance of `provider.ContextEngine`. During `AssembleForAgent()`, dynamically extract `activeFiles` from structured task metadata, calculate an available token budget, call `GetRepoMap()`, and prepend the output to the LLM's user prompt under `=== Repository Map ===`.

**Tech Stack:** Go 1.23.

---

## Current Problems

| Problem | Location | Impact |
| :--- | :--- | :--- |
| **Tunnel Vision** | `prompt/assembler.go` | The LLM only sees isolated code snippets from `retriever.RetrieveContext`. It doesn't know where those snippets live relative to the rest of the project. |
| **Missing Dependencies**| Agent Generation | When writing code, the AI guesses import paths or helper functions because it cannot see the project's dependency graph. |
| **Silent Context Overflow**| Prompt Builder | Blindly appending text components to the prompt causes token overflow if budgets are not arbitrated centrally. |

---

## Migration Shape

```text
1. Wire ContextEngine into main.go Initialization        → Task 1
2. Extract Active Files from Structured Workflow State   → Task 2
3. Inject Repository Map with Token Budget Arbitration   → Task 3
4. Update Unit Tests for PromptAssembler                 → Task 4
```

---

## Task 1: Wire ContextEngine into Orchestrator Initialization

> Addresses: Dependency Injection of the new Engine and correct App bootstrap.

**Files:**
- Modify: `server/internal/orchestrator/prompt/assembler.go`
- Modify: `server/cmd/api/main.go`

- [x] **Step 1: Update `PromptAssembler` Struct**
  - Add field: `ctxEngine provider.ContextEngine` to `PromptAssembler`.
  - Update `NewPromptAssembler` and `NewPromptAssemblerWithRules` constructors to accept `ctxEngine`.
- [x] **Step 2: Initialize Provider in `main.go`**
  - Inside `server/cmd/api/main.go`, derive `cacheDbPath` using `filepath.Join(viper.GetString("DATA_ROOT"), "cache.db")` (or the equivalent data directory config).
  - Instantiate `engine, err := provider.NewProvider(workspaceRoot, cacheDbPath)`.
  - Pass `engine` into the `NewPromptAssembler` factory.
  - Ensure `engine.Close()` is deferred gracefully.

---

## Task 2: Extract Active Files from Structured Workflow State

> Addresses: Making the Repo Map "Personalized" safely without fragile regex scraping.

**Files:**
- Modify: `server/internal/orchestrator/prompt/assembler.go`

- [x] **Step 1: Extract from Semantic Snippets**
  - In `AssembleForAgent`, after calling `a.retriever.RetrieveContext()`, iterate through `snippets` and collect unique `snippet.Filepath` values.
- [x] **Step 2: Extract from Structured Task Metadata**
  - Do NOT scrape prose in `SpecsMD`. Extract target files strictly from structured metadata.
  - Check workflow execution state or `task.Analysis.ExecutionPlan` (if structured) to identify the assigned file targets for the current step.
  - Combine and deduplicate these files into an `activeFiles []string` array to pass into `GetRepoMap`.

---

## Task 3: Inject Repository Map with Token Budget Arbitration

> Addresses: Preventing model window overflow by dynamically budgeting tokens across OpenSpec, Snippets, and RepoMap.

**Files:**
- Modify: `server/internal/orchestrator/prompt/assembler.go`

- [x] **Step 1: Dynamic Token Arbitration**
  - Before calling `GetRepoMap`, calculate the estimated token load of the compiled `OpenSpec`, `snippets`, and `memories` (or use a character-based heuristic `chars/4` for speed).
  - Calculate a dynamic `maxMapTokens` allocation: `maxMapTokens = TOTAL_BUDGET - usedTokens`.
  - Impose a safe absolute ceiling (e.g., max 2048 tokens for the Repo Map) to ensure logic remains balanced.
- [x] **Step 2: Call `GetRepoMap`**
  - Call `repoMap, err := a.ctxEngine.GetRepoMap(activeFiles, maxMapTokens)`.
  - If `err != nil`, log a warning but DO NOT crash the prompt assembly (graceful degradation).
- [x] **Step 3: Append to User Prompt**
  - Prepend the `repoMap` string to the `user` message block, formatted neatly under `=== Repository Structure ===`.

---

## Task 4: Update Unit Tests

> Addresses: Ensuring the Assembler doesn't break due to the new dependency and mock compilation failures.

**Files:**
- Modify: `server/internal/orchestrator/prompt/assembler_test.go`

- [x] **Step 1: Create `MockContextEngine`**
  - Implement a fake `ContextEngine` that satisfies the exact interface defined in `provider.go`.
  - `GetRepoMap(activeFiles []string, maxTokens int) (string, error)` returns a dummy skeleton string.
  - **Crucial:** Implement `Close() error { return nil }` so the mock correctly satisfies the interface.
- [x] **Step 2: Update Existing Tests**
  - Fix all existing calls to `NewPromptAssembler` in `assembler_test.go` by passing `MockContextEngine`.
- [x] **Step 3: Assert Repo Map Injection**
  - Add test `TestPromptAssembler_InjectsRepoMap`.
  - Assert that the final prompt string contains `=== Repository Structure ===`.
- [x] **Step 4: Run tests and check task finish**
  ```bash
  go test ./internal/orchestrator/prompt/... -v
  ```
