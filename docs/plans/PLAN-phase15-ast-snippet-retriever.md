# AST-Backed Snippet Retriever — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Eliminate the legacy Regex-based `FileContextRetriever` (`server/internal/orchestrator/codecontext/retriever.go`) to resolve technical debt, hardcoded logic, and Clean Code violations. Instead, upgrade the Phase 13 `ContextEngine` to natively support micro-context retrieval using AST boundaries and SQLite caching.

**Architecture:**
1. Upgrade Tree-sitter `Extractor` to capture the `EndLine` of function definitions.
2. Expose `RetrieveContext` via `provider.ContextEngine`.
3. Use the SQLite symbol cache to quickly locate relevant functions without expensive disk-walks.
4. Purge the `server/internal/orchestrator/codecontext` folder completely.

---

## Current Technical Debt

| Violation | Location | Fix in Phase 15 |
| :--- | :--- | :--- |
| **Fragile Regex** | `retriever.go:81` | Replace with robust AST (Tree-sitter) parsing. |
| **Disk I/O Abuse** | `retriever.go:136` | Replace `filepath.WalkDir` with SQLite queries over `FileMeta` and `Tag`. |
| **Hardcoded Configs** | `retriever.go:141` | Eliminate hardcoded ignored directories/extensions. |

---

## Migration Shape

```text
1. Upgrade AST Tag Model to support Full Boundaries      → Task 1 (Done)
2. Implement Lexical Search over SQLite Cache            → Task 2 (Done)
3. Implement `RetrieveContext` in ContextEngine          → Task 3 (Done)
4. Wire to Assembler & Eradicate Legacy Code             → Task 4 (Pending deletion of legacy folder)
```

---

## Task 1: Upgrade AST Tag Model to support Full Boundaries

> Addresses: Accurate Snippet clipping.

**Files:**
- Modify: `server/internal/context/source/scanner.go`
- Modify: `server/internal/context/source/cache.go`
- Modify: `server/internal/context/symbol/extractor.go`

- [x] **Step 1: Expand `Tag` Struct**
  - Add `EndLine int` to `source.Tag` struct.
- [x] **Step 2: Update SQLite Schema / Serialization**
  - Tags are serialized as JSON payload into `tags_json TEXT` column in `file_cache` table. Ensure `EndLine` is written/read correctly.
- [x] **Step 3: Capture End Boundaries in Tree-sitter**
  - In `symbol/extractor.go`, when parsing a node, extract `Node.EndPoint().Row` and map it to `EndLine`.
  - For `fallback` Regex extractor, default `EndLine` to `-1` or ignore.

---

## Task 2: Implement Fast Lexical Search over SQLite

> Addresses: Replacing the expensive `WalkDir` with a lightning-fast indexed search.

**Files:**
- Modify: `server/internal/context/source/search.go`

- [x] **Step 1: Tokenize Query**
  - Implement query tokenizer and helper search functions in `search.go`.
- [x] **Step 2: Add `SearchTags` Method**
  - Implement `SearchTags(terms []string, limit int) ([]Tag, error)` on `source.Cache`.
  - Score tag matches using terms and path mappings, returning sorted definitions (`kind = 'def'`).

---

## Task 3: Implement RetrieveContext in ContextEngine

> Addresses: Satisfying the interface for the Orchestrator.

**Files:**
- Modify: `server/internal/context/provider/provider.go`

- [x] **Step 1: Extend Interface**
  - Add `RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error)` to `ContextEngine` interface.
- [x] **Step 2: Implement Method**
  - Inside `Provider.RetrieveContext()`, call `c.cache.SearchTags` based on the query.
  - For the top tags, read file contents and extract lines between `Tag.Line` and `Tag.EndLine`.
  - Package these lines into `models.ContextSnippet`.
- [x] **Step 3: Score Fallbacks**
  - If no tags match the query, gracefully return an empty slice `[]`.

---

## Task 4: Complete Refactoring & Eradicate Legacy Code (Option A)

> Addresses: Clean Code cleanup, eliminating Tech Debt, and establishing a Single Source of Truth.

**Files:**
- Modify: `server/internal/context/provider/provider.go`
- Modify: `server/internal/orchestrator/steps/analyze.go`
- Modify: `server/internal/orchestrator/llmrunner/runner.go`
- Modify: `server/internal/orchestrator/prompt/assembler.go`
- Modify: `server/cmd/api/main.go`
- Delete: `server/internal/orchestrator/codecontext/`

- [x] **Step 1: Migrate Workspace Context Key**
  - Define `WorkspaceRootKey ContextKey = "retriever_workspace_root"` inside `server/internal/context/provider/provider.go`.
  - Update `analyze.go` and `runner.go` to import `provider` and use `provider.WorkspaceRootKey`.
- [x] **Step 2: Update Constructor Wire-up**
  - Change `PromptAssembler` fields and constructors to point directly to `ctxEngine`.
  - Remove all legacy retriever setup.
- [x] **Step 3: Delete Legacy Files**
  - Run `rm -rf server/internal/orchestrator/codecontext` to permanently destroy the outdated module.
- [x] **Step 4: Run Tests & Compile**
  - Ensure all mock implementations of `ContextEngine` implement `RetrieveContext`.
  - Run `go test ./...` to verify the codebase is completely free of the old Retriever dependencies and compiles successfully.
