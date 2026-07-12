---
snapshot_date: 2026-07-06
scope: Context Management Engine Architectural Proposal
confidence: High (Derived from conceptual analysis)
status: Implemented (Mapping to docs/features/engineering/01-context-management.md — audited 2026-07-12, already built)
---
# AI Coding Agent Context Management MVP
### Go + Tree-sitter + Repository Map

> **Goal**
>
> Build a lightweight, multi-language Context Management system for an AI Coding Agent.
>
> The focus is **providing the right context to the LLM**, not building a complete IDE or compiler.

# 1. Why Context Management?

Large Language Models cannot read an entire repository.

Example:

- 8,000 source files
- 2M+ LOC
- Hundreds of packages
- Thousands of symbols

Even with large context windows, sending the entire repository is impossible.

Therefore the problem becomes:

> Given a repository with thousands of files, **which files and symbols should be sent to the LLM?**

That is the responsibility of the Context Management layer.

---

# 2. Design Philosophy

This project is **NOT** trying to build:

- an IDE
- a compiler
- a semantic analyzer
- a refactoring engine

Instead it focuses on:

> Supplying the minimum amount of useful context that allows an LLM to understand a project.

Architecture:

```
Repository
      │
      ▼
Context Builder
      │
      ▼
LLM
```

---

# 3. Design Principles

The architecture follows several core principles.

## Multi-language First

The Context Engine should support multiple languages.

Language-specific logic belongs only inside the parser layer.

---

## Incremental Update (Future Evolution)

*Note: The Current MVP uses On-Demand `mtime` Caching instead of active background updating.*

Never rebuild the entire repository after every change.

Instead:

```
Modified File

↓

Reparse

↓

Update Index

↓

Update Repository Map
```

---

## Token Efficient

Repository context should consume as few tokens as possible.

Priority:

```
Repository Map

↓

Relevant Symbols

↓

Relevant Source Files
```

Never send unnecessary files.

---

## Parser Independent (Future Evolution)

*Note: The Current MVP relies purely on Tree-sitter and a simple Lexer Fallback.*

The Context Engine should not depend on Tree-sitter itself.

Tree-sitter is only one possible parser.

Future replacements may include:

- LSP
- Compiler APIs
- Native language parsers

without changing the architecture.

---

## LLM Friendly

Repository Map should be easy for an LLM to read.

Avoid sending:

- AST
- CST
- JSON dumps
- compiler data

Instead generate structured plain text.

---

# 4. Why Not LSP?

Language Server Protocol provides deep semantic analysis but introduces significant complexity.

Typical LSP deployment requires:

- language servers
- RPC communication
- workspace synchronization
- cache management
- language-specific implementations

For an MVP, this complexity is unnecessary.

The goal is simply to know:

- what files exist
- what symbols exist
- where symbols are located

Tree-sitter is sufficient.

---

# 5. Design Note: Inspiration from Aider Repository Map

*Note: The following details represent conceptual inspiration from Aider. Specific technical parameters (like token pruning tolerance, specific cache formats, or exact heuristic weights) are design notes for future implementation, not established features of our current internal repository.*

The design is inspired by Aider's Repository Map.

Repository Map is **NOT** source code.

It contains only high-level project metadata:

- classes
- structs
- interfaces
- functions
- methods
- signatures

Example:

```
internal/auth/login.go

func Login(ctx context.Context, req LoginRequest)

func Logout(ctx context.Context)

type LoginRequest struct
```

The LLM first understands the repository structure.

Only when necessary does it load the actual source file.

---

# 6. Overall Architecture

```
Repository
      │
      ▼
Source Discovery
      │
      ▼
On-Demand Cacher
      │
      ▼
Tree-sitter Parser
      │
      ▼
Syntax Tree
      │
      ▼
Symbol Extractor
      │
      ▼
Repository Index
      │
      ▼
Repository Map Builder
      │
      ▼
Context Provider
      │
      ▼
Prompt Builder
      │
      ▼
LLM
```

---

# 7. Components

---

## 7.1 Source Discovery

Responsibilities:

- walk repository
- ignore unnecessary directories
- detect language
- collect source files

Ignored directories:

```
.git
vendor
node_modules
dist
build
coverage
tmp
```

Output:

```
internal/auth/login.go

internal/auth/jwt.go

internal/storage/user.go

cmd/server/main.go
```

---

## 7.2 On-Demand Caching (mtime)

Instead of using a resource-intensive background daemon (File Watcher), parsing is done **On-Demand**.

When a Repository Map is requested:
1. Stat all files to get their Modification Time (`mtime`).
2. If `mtime` hasn't changed since the last parse -> Load AST tags from Cache.
3. If `mtime` changed -> Reparse the file and update Cache.

Advantages:
- Zero background CPU usage.
- No cross-platform watcher bugs.
- Instantly syncs with the exact state of the disk when the LLM makes a request.

---

## 7.3 Tree-sitter Parser

Responsible only for parsing source code.

Output:

Syntax Tree.

Example:

```go
func Login(ctx context.Context) error {

}
```

↓

Syntax Tree

↓

Function Declaration

Advantages:

- fast
- incremental
- multi-language

---

## 7.4 Symbol Extractor (Definitions & References)

This is the core component.

Using **Tree-sitter Query Language (.scm files)**, the extractor must capture **TWO** critical tag types:

1. `def` (Definitions): Where a symbol is created (`func Login()`, `class User`).
2. `ref` (References): Where a symbol is called (`auth.Login()`, `new User()`).

Example:

```
queries/

go/
    symbols.scm

python/
    symbols.scm
```

If you only capture `def`, the Repository Map becomes a "blind" Table of Contents. By capturing `ref`, we can build a connected Dependency Graph to see how files interact.

Adding a new language only requires:

- Tree-sitter grammar
- symbols.scm

No Go code changes.

---

## 7.5 Fallback Parser

Tree-sitter requires language grammars.

For unsupported languages, provide a simple Regex-based Symbol Extractor.

Capabilities:

- detect function
- detect class
- detect interface

Accuracy is lower but ensures the Agent is never completely blind.

---

## 7.6 Repository Index

Stores all extracted symbols.

Example:

```
login.go

- Login()
- Logout()

jwt.go

- Generate()
- Verify()

user.go

- UserRepository
```

This is an internal database.

It is **NOT** sent directly to the LLM.

---

## 7.7 Repository Map Builder (PageRank & Token Pruning)

Generates a compact "Skeleton Map" of the repository, showing structures and signatures but **no function bodies**.

Example:

```
internal/auth/login.go
func Login(...)
func Logout(...)
--------------------------------
internal/auth/jwt.go
type JWTManager
func Generate(...)
func Verify(...)
```

### MultiDiGraph & PageRank Ranking

For repositories with thousands of symbols, simple heuristics will still blow up the LLM Token window. 
Instead, we apply Google's **PageRank** algorithm:

1. **Build Graph:** Connect File A to File B if File A contains a `ref` calling a `def` in File B. Edge weight is calculated as `sqrt(number_of_calls)`.
2. **Personalize:** Boost the starting score of files currently open in the IDE or mentioned in the user's chat prompt.
3. **Propagate:** Run PageRank to distribute importance from the active files to the peripheral utility files.

### Token Optimization (Binary Search)

If the LLM context limit for the Repo Map is 1024 tokens:
1. Sort all tags by their PageRank score.
2. Use **Binary Search** to test different slice sizes of the top N tags.
3. Render the AST and count tokens until the output fits perfectly under the 1024 token limit (with ~15% tolerance).

This guarantees the LLM gets the most mathematically relevant context without ever exceeding the token limit.

---

## 7.8 Context Provider

When the user requests:

```
Refactor Login API
```

The Context Provider:

1. locate Login
2. locate login.go
3. load Repository Map
4. load related symbols
5. build prompt

Entire repository is never loaded.

---

## 7.9 Prompt Builder

Example prompt layout:

```
Repository Summary

Relevant Symbols

Relevant Files

Current File

User Request
```

↓

LLM

---

# 8. Internal Data Model

```go
type Repository struct {
    Files map[string]*SourceFile
}

type SourceFile struct {
    Path     string
    Language string
    Symbols  []*Symbol
}

type Symbol struct {
    ID          string
    ParentID    string

    Name        string
    Kind        SymbolKind
    Signature   string

    Language    string
    FilePath    string

    Scope       string

    StartLine   int
    EndLine     int

    Exported    bool
}
```

Example hierarchy:

```
UserService

├── Login()

├── Logout()
```

instead of two unrelated methods.

---

# 9. Repository Map Example

```
internal/auth/login.go

func Login(ctx context.Context, req LoginRequest)

func Logout()

--------------------------------

internal/auth/jwt.go

type JWTManager

func Generate()

func Verify()

--------------------------------

internal/storage/user.go

type UserRepository

func FindByID()

func Save()
```

This is what gets sent to the LLM.

---

# 10. Workflow

## Initial Repository Scan

```
Repository

↓

Walk Files

↓

Tree-sitter

↓

Extract Symbols

↓

Repository Index

↓

Repository Map
```

---

## On-Demand Map Generation (User Request)

```
User Request
↓
Scan all files (get mtime)
↓
Cache Hit -> Load Tags | Cache Miss -> Reparse (Tree-sitter)
↓
Build MultiDiGraph (defs & refs)
↓
Run Personalized PageRank
↓
Binary Search Token Pruning
↓
Render Compact Repo Map
↓
Prompt Builder
↓
LLM
```

---

# 11. Suggested Technologies

Parser

- go-tree-sitter

Cache & Storage

- SQLite (store tags keyed by `mtime`)

Graph Math

- go-graph / gonum (for MultiDiGraph and PageRank)

Token Counter

- tiktoken-go

Ignore

- .gitignore parser (optional)

---

# 12. Advantages

- lightweight
- easy to implement
- multi-language
- fast
- incremental
- low token usage
- parser independent
- LLM friendly

---

# 13. Limitations

MVP does NOT understand:

- type inference
- references
- interface implementations
- call graph
- semantic analysis
- compiler diagnostics

These features can be added later.

---

# 14. Future Roadmap

## Phase 1 (MVP)

- Repository Scanner (mtime caching)
- Tree-sitter Parser (`def` and `ref` extraction)
- Graph Builder & PageRank
- Binary Search Token Pruning
- AST Skeleton Renderer

Deliverable:

MVP capable of analyzing 1M+ LOC without token overflow.

---

## Phase 2

- Fallback Lexer (for languages missing Tree-sitter grammars)
- Session Memory
- Working Set (Chat Context injection into PageRank)

---

## Phase 3

- Vector Database (Embedding Search)
- Semantic Search

---

## Phase 4

- Session Memory
- Working Set
- Recent Files

---

## Phase 5

- Embedding Search
- Semantic Search
- Vector Database

---

## Phase 6

Optional

- LSP Integration
- Call Graph
- Type Analysis

---

# 15. Suggested Go Project Layout

```
internal/

    context/

        source/
            scanner.go
            cache.go

        parser/
            treesitter.go
            fallback.go

        symbol/
            extractor.go
            symbol.go

        index/
            repository.go

        repomap/
            builder.go
            formatter.go
            ranking.go

        provider/
            context_provider.go

        prompt/
            builder.go
```

---

# 16. Future Extensions

The architecture is intentionally designed for extensibility.

Future capabilities include:

- Dependency Graph
- Call Graph
- Symbol References
- Git Diff Context
- Session Memory
- Working Set
- Incremental Repository Index
- Embedding Search
- Vector Database
- Optional LSP Integration

These additions can be implemented without changing the core architecture.

---

# 17. Core Design Idea

The most important principle is:

> **The LLM should never read the entire repository.**

Instead, the LLM receives only:

- Repository Map
- User Request
- Current File
- Relevant Files (if necessary)

The Repository Map serves as the **table of contents** for the project.

It allows the LLM to understand the overall structure of the repository while consuming very few tokens.

Only when additional information is required does the Context Provider load the actual source code.

This is the same core philosophy used by Aider's Repository Map, simplified into a lightweight architecture suitable for a Go implementation.
````
