# AST-Based Editing — Backlog & Research Notes

> **Status:** Postponed / Backlog (Originally scheduled for Phase 12)  
> **Goal:** Leverage Abstract Syntax Trees (AST) via Tree-sitter to perform semantically aware code modifications, replacing textual search-and-replace with structured syntax edits.

## 1. Why AST-Based Editing?
Text-based patch formats (Unified Diff, Search & Replace) are prone to failures due to:
- Formatting inconsistencies (spaces vs tabs, line endings).
- Ambiguity in search blocks (identical patterns matching multiple times in a file).
- LLM output line count mismatches.

By parsing code into an AST:
1. The engine understands language structures (functions, structs, loops).
2. It can identify the exact node to target (e.g., "Add parameter `X` to function `foo`").
3. Edits are guaranteed to be syntactically valid before being written back.

---

## 2. Technical Stack & Dependencies
- **Parser Engine:** Tree-sitter (`github.com/smacker/go-tree-sitter`) or Go standard library `go/ast` (specifically for Go files).
- **Target Languages:** Go (`go.mod`), TypeScript/React (`web/`).

---

## 3. Future Scaffolding Checklist

### Step 1: Define `ASTEditApplier`
- Implement the `PatchEngine` interface.
- Return an `"AST editing not implemented"` error for non-supported languages or configurations.

### Step 2: Parser Integration
- Load specific language grammars (e.g., Go tree-sitter bindings).
- Parse the target source file into a concrete syntax tree.

### Step 3: Edit Node Target Identification
- Define a serialization format for AST instructions (e.g., JSON syntax specifying `Action`, `NodePath`, `NewCode`).
- Locate the node using node path selectors.

### Step 4: Tree Modification & Serialization
- Perform tree mutation.
- Serialize the modified tree back to source text.
- Run formatter (e.g., `gofmt` or `prettier`) to ensure clean code style.
