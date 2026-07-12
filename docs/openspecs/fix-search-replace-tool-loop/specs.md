# Specs: Fix Search/Replace Tool Infinite Loop

## Added Requirements

### REQ-001: Actionable error on search_replace for non-existent files
> ✅ Status: Completed

**Scenario:**
- WHEN the LLM calls `search_replace` with `search != ""` on a file that does not exist
- THEN the tool returns a diagnostic with severity "error" containing guidance:
  `"File '<path>' does not exist. To create it, use the create_file tool, or call search_replace with an empty search string to write initial content."`
- AND the tool returns `Success: false`
- AND the error message does NOT contain raw OS error text like "no such file or directory"

### REQ-002: Prompt informs LLM about non-existent affected files
> ✅ Status: Completed

**Scenario:**
- WHEN `ReadAffectedFileContent` returns `("", false)` for a file in `AffectedFiles`
- THEN the `### Workspace Affected Files ###` section includes a placeholder block:
  ```
  --- <display_path> [NEW FILE — does not exist yet] ---
  This file needs to be created. Use the `create_file` tool.
  ```
- AND the LLM receives this information before making its first tool call

### REQ-003: Tool loop circuit-breaker for repeated identical failures
> ✅ Status: Completed

**Scenario:**
- WHEN the LLM calls the same tool with the same `path` argument and receives a failure response
- AND this identical failure has occurred 2 consecutive times already
- THEN the tool loop injects a corrective user message:
  `"You have called <tool_name> on <path> multiple times without success. The file likely does not exist. Use create_file to create it first, then use search_replace to modify it."`
- AND the tool call is skipped (the corrective message replaces the tool result)
- AND the iteration counter is NOT incremented for skipped calls

### REQ-004: Empty search block overwrites instead of appending
> ✅ Status: Completed

**Scenario:**
- WHEN `ApplySearchReplace` processes a block where `search == ""`
- AND the target file already exists with content "old content\n"
- THEN the file content is replaced entirely with the `replace` block value
- AND the previous content is NOT appended to

**Scenario (new file):**
- WHEN `ApplySearchReplace` processes a block where `search == ""`
- AND the target file does not exist
- THEN the file is created with the `replace` block content
- AND parent directories are created as needed

## Modified Requirements

### REQ-M01: search_replace tool allows creation with empty search
> ✅ Status: Completed

**Scenario (no behavior change — verify existing):**
- WHEN the LLM calls `search_replace` with `search == ""` on a file that does not exist
- THEN the tool creates the file with the `replace` content
- AND returns `Success: true`
- AND `FilesChanged` includes the path

### REQ-M02: Tool loop respects max iterations correctly
> ✅ Status: Completed

**Scenario:**
- WHEN the tool loop reaches `MaxIterations` (default 6)
- THEN it returns an error `"exceeded max iterations (6)"`
- AND circuit-breaker skipped calls do NOT count toward the iteration limit

## Removed Requirements
- None
