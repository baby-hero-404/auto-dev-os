# Context Engine MVP — Implementation Plan

> **For agentic workers:** Use `subagent-driven-development` or executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Implement a multi-language Context Management MVP (Repository Map) that provides an LLM with a structural codebase overview without exceeding token limits. The engine uses on-demand SQLite caching, Tree-sitter for `def`/`ref` extraction, and Personalized PageRank for mathematical context prioritization.

**Tech Stack:** Go 1.23, `github.com/smacker/go-tree-sitter`, `gonum.org/v1/gonum/graph`, `github.com/mattn/go-sqlite3`, `github.com/pkoukk/tiktoken-go`.

---

## Current Problems

| Problem | Location | Impact |
| :--- | :--- | :--- |
| **Token Overflow** | Prompt Builder | Sending full files or blind Tables of Contents easily breaches LLM limits (e.g., sending all 10,000 functions). |
| **Blind Context** | LLM Agent | The Agent guesses file names and functions, causing hallucinated imports because it cannot see cross-file dependencies. |
| **Performance Drag** | File Parser | Parsing the entire repo on every change is CPU-intensive and blocks the workflow. |

---

## Migration Shape

```text
1. Build Infrastructure & SQLite Cache (mtime)           → Task 1
2. Implement AST Extractor (Tree-sitter def/ref)         → Task 2
3. Implement Graph Builder & PageRank Algorithm          → Task 3
4. Implement Token Pruning & Skeleton Renderer           → Task 4
5. Define ContextProvider API & Run Integration Tests    → Task 5
```

---

## Task 1: Build Infrastructure & On-Demand Cache

> Addresses: Performance drag via instant `mtime` cache retrieval.

**Files:**
- Create: `internal/context/source/scanner.go`
- Create: `internal/context/source/cache.go`

- [x] **Step 1: Define Structs**
  ```go
  type FileMeta struct {
      Filepath string
      Mtime    int64
  }
  type Tag struct {
      Name     string
      Kind     string // "def" or "ref"
      Line     int
      Filepath string
  }
  ```
- [x] **Step 2: Implement Repository Scanner (`scanner.go`)**
  - Walk directory tree ignoring `.git`, `node_modules`, `vendor`.
  - Return slice of `FileMeta`.
- [x] **Step 3: Implement SQLite Cache (`cache.go`)**
  - Initialize DB connection (using `mattn/go-sqlite3`).
  - Implement `GetTagsIfFresh(filepath string, mtime int64) ([]Tag, bool)`
  - Implement `SaveTags(filepath string, mtime int64, tags []Tag)` (serialize tags to JSON/Gob).
- [ ] **Step 4: Run tests and commit**
  ```bash
  go test ./internal/context/source/... -v
  git commit -m "feat(context/source): implement repository scanner and sqlite mtime cache"
  ```

---

## Task 2: Implement AST Extraction & Fallback

> Addresses: Finding exactly where symbols are defined (`def`) and called (`ref`).

**Files:**
- Create: `internal/context/parser/treesitter.go`
- Create: `internal/context/symbol/extractor.go`
- Create: `internal/context/parser/fallback.go`

- [x] **Step 1: Setup Tree-sitter & Queries**
  - Initialize `smacker/go-tree-sitter`.
  - Create directory `queries/go`, `queries/python` etc., with `symbols.scm` files targeting `name.definition.` and `name.reference.` tags.
- [x] **Step 2: Implement `symbol/extractor.go`**
  - Read source code, execute `.scm` queries.
  - Map captures to `Tag` struct (filtering out comments/strings).
- [x] **Step 3: Implement `fallback.go` (Lexer Regex)**
  - For unsupported file extensions, use Regex `[a-zA-Z_][a-zA-Z0-9_]*` to extract all words as `ref` tags to prevent crashes.
- [ ] **Step 4: Run tests and commit**
  ```bash
  go test ./internal/context/symbol/... ./internal/context/parser/... -v
  git commit -m "feat(context/symbol): implement tree-sitter extraction and lexer fallback"
  ```

---

## Task 3: Graph Construction & PageRank

> Addresses: Blind context by mathematically linking dependencies to the active chat prompt.

**Files:**
- Create: `internal/context/repomap/builder.go`
- Create: `internal/context/repomap/ranking.go`

- [x] **Step 1: Implement MultiDiGraph Builder**
  - Initialize a directed graph using `gonum/graph/multi`.
  - Iterate tags: For each `ref` in File A calling `def` in File B, add edge `A -> B`.
- [x] **Step 2: Calculate Edge Weights**
  - Weight = `math.Sqrt(float64(number_of_calls))`.
  - Apply heuristic multipliers (e.g., `_private` methods * 0.1).
- [x] **Step 3: Implement Personalized PageRank (`ranking.go`)**
  - Accept `activeFiles []string` (files mentioned in user prompt).
  - Assign starting personalization scores heavily weighted towards `activeFiles`.
  - Run `gonum` PageRank to generate a map of `map[string]float64` (Tag -> Score).
- [ ] **Step 4: Run tests and commit**
  ```bash
  go test ./internal/context/repomap/... -run "TestGraph" -v
  git commit -m "feat(context/repomap): implement dependency graph and personalized pagerank"
  ```

---

## Task 4: Token Pruning & AST Skeleton Renderer

> Addresses: Token overflow by slicing the Repo Map perfectly.

**Files:**
- Create: `internal/context/repomap/pruning.go`
- Create: `internal/context/repomap/formatter.go`

- [x] **Step 1: Implement Binary Search Pruner**
  - Sort all tags by their PageRank score.
  - Use binary search (`lower=0`, `upper=len(tags)`) to slice Top N tags.
  - Call formatter to generate text, count tokens using `tiktoken-go`.
  - Stop when tokens are within 15% of `maxTokens` (e.g., 1024).
- [x] **Step 2: Implement Skeleton Renderer (`formatter.go`)**
  - Group pruned tags by Filepath.
  - Reconstruct source file lines holding the `def` tags.
  - Exclude any lines inside `{ ... }` blocks, preserving structural indentation only.
- [x] **Step 3: Run tests and check task finish**
  ```bash
  go test ./internal/context/repomap/... -run "TestPruning" -v
  git commit -m "feat(context/repomap): implement binary search token pruning and skeleton formatter"
  ```

---

## Task 5: Define API Provider & Run Integration Tests

> Addresses: Exposing the engine to the orchestrator safely.

**Files:**
- Create: `internal/context/provider/provider.go`
- Create: `internal/context/provider/provider_test.go`

- [x] **Step 1: Define Interface**
  ```go
  type ContextEngine interface {
      GetRepoMap(activeFiles []string, maxTokens int) (string, error)
  }
  ```
- [x] **Step 2: Implement Orchestrator Binding**
  - Wire Scanner -> Cache -> Extractor -> Builder -> Pruner -> Formatter together.
- [x] **Step 3: Latency & Leakage Tests**
  - Create a mock repository of 1,000 files.
  - Assert Cold Start < 10 seconds.
  - Assert Hot Cache < 50ms.
  - Assert no "{ body }" logic is leaked in the `GetRepoMap` output string.
- [ ] **Step 4: Run tests and commit**
  ```bash
  go test ./internal/context/provider/... -v
  git commit -m "feat(context/provider): implement context provider api and strict latency integration tests"
  ```
