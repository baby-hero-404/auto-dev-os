# Specs: Fix Analyze Tool Path Resolution

## Modified Requirements

### REQ-M01: Strict Path Management Abstraction
> ✅ Status: Implemented

**Scenario:**
- WHEN `AnalyzeStep` attempts to discover workspace repositories or search for files.
- THEN it MUST use `paths.OSWorkspacePaths` domain methods (e.g. `TaskRoot`, `RepoMain`) rather than manual string building or `filepath.Join()`.
- AND the resolved output must correctly point to the `code/repos/<repo-name>/main` directory for any repository roots.

### REQ-M02: Analyze Sandbox Logging Accuracy
> ✅ Status: Implemented

**Scenario:**
- WHEN `AnalyzeStep` runs tools like `list_files` or `read_file`.
- THEN the sandbox must NOT run arbitrary fallback commands (`find .` or `grep -RIn`) that pollute the LLM prompt with workspace metadata files.
- AND the `analyze_test.go` suite MUST validate the core tool output directly without relying on testing side-effects of sandbox log scraping.
