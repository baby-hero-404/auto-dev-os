# Code Test Coverage Plan — Missing Edge Cases

**Created:** 2026-07-02  
**Scope:** Supplement unit and integration test coverage for critical core engines identified during the Feature Review phase.  
**Priority:** 🟡 Medium — Ensures stability against LLM output hallucination

---

## 1. Search & Replace Parser Tests (`search_replace_test.go`)

The Search & Replace (S&R) parser is the primary mechanism for the LLM to apply code changes. LLMs frequently format these blocks unpredictably. The current test suite (`1.4KB`) only tests the "happy path". 

We need to add the following edge cases to `TestParseSearchReplace`:

### Task List:
- [x] **Multi-block parsing:** A single patch string containing 2 or more distinct `<<<<<<< SEARCH` blocks.
- [x] **File Creation (Empty SEARCH):** A block with an empty SEARCH section (used when Aider style creates a new file).
- [x] **File Deletion/Clearing (Empty REPLACE):** A block with an empty REPLACE section (used to delete code).
- [x] **Malformed Markers:** Output where `<<<<<<< SEARCH` is missing a few brackets, or `=======` is missing. Ensure the parser fails gracefully or ignores it rather than panicking.
- [x] **File Path Heuristics:** Test extraction of the filepath when it's formatted as:
  - `File: path/to/file.go`
  - `` `path/to/file.go` ``
  - `file: path/to/file.go`

## 2. Patch Validator Tests (`validator_test.go`)

Ensure the validator catches S&R edge cases correctly.

### Task List:
- [x] **Duplicate Matches:** Test that `ValidateSearchReplace` fails if the `SEARCH` string appears exactly 2 times in the target file (ambiguous match).
- [x] **No Match:** Test that it fails if the `SEARCH` string does not exist in the file.

## 3. Patch Engine Self-Healing Integration Test (`applier_test.go` or `search_replace_test.go`)

We need to verify that when a patch fails validation, the error is correctly bubbled up so the workflow engine can trigger a "Fix" cycle.

### Task List:
- [x] **Failed Validation Bubbling:** Write a test where `ApplySearchReplace` is given a patch that fails validation (e.g., target string not found), and assert that the `PatchEngine.Apply()` returns the specific validation error so the LLM gets feedback.
- [x] **Newline Normalization Test:** Test that a `SEARCH` block with `\r\n` correctly matches a file on disk with `\n`.

---

## Next Steps

To execute this plan:
1. Open `server/internal/orchestrator/patch/search_replace_test.go` and implement the table-driven tests for the parser.
2. Run `cd server && go test ./internal/orchestrator/patch/... -v` to ensure they pass.
3. If any parser logic fails the new edge cases, fix the parser logic in `search_replace.go`.
