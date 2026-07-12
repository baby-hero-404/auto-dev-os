# Proposal: Fix Search/Replace Tool Infinite Loop on Non-Existent Files

## Why

During agentic coding and fix steps, the LLM frequently enters an unproductive loop where it
calls the `search_replace` tool repeatedly on files that **do not exist yet**. This wastes
tokens and produces no working code changes. The root cause is a chain of 3 interacting bugs:

1. **Silent file absence in prompt context.** `readAffectedFileContent` (sandbox.go:98) returns
   `("", false)` for files that don't exist on disk. The caller in `llmrunner/runner.go:67`
   silently skips these files, so the `### Workspace Affected Files ###` section in the prompt
   never mentions them. The LLM sees the file listed in the Execution Manifest but cannot see
   its content, leaving it uncertain whether the file is empty or missing.

2. **Unhelpful error message from `search_replace` tool.** When the LLM calls `search_replace`
   with `search != ""` on a file that doesn't exist, the tool returns:
   `"cannot read file: open <path>: no such file or directory"`.
   This generic OS error does not tell the LLM *what to do instead* (use `create_file`, or use
   `search_replace` with `search=""` to write the full content). The LLM retries the same call,
   or tries slight variations of the search block, entering a futile loop.

3. **No circuit-breaker in `toolloop.go`.** The `RunToolLoop` (toolloop.go:46) has a hard
   iteration cap of 6–8 but no detection for *repeated identical failures*. If the LLM calls
   `search_replace` on the same non-existent file 6 times in a row, every call burns tokens and
   returns the same generic error.

### Secondary Issue: `search==""` append-vs-replace ambiguity

In `patch/search_replace.go:133-135`, when `search == ""`, the code performs `content += replace`
(append). If the file already exists and has content, this appends the replacement at the end
instead of replacing the entire file. This contradicts the tool description which says "replace
the entire file contents" and can corrupt files on retry.

## What Changes

### Issue 1: Silent file absence in prompt — inform the LLM which files are new

- In `llmrunner/runner.go`, when `ReadAffectedFileContent` returns `("", false)`, emit a
  placeholder block telling the LLM the file does not exist and should be created.

### Issue 2: Unhelpful `search_replace` error on non-existent files

- In `server/internal/tool/tools/search_replace.go`, when `os.IsNotExist(err) && search != ""`,
  return an actionable diagnostic:
  `"File does not exist. Use create_file to create it, or call search_replace with search=\"\"
  to write the full content."`

### Issue 3: No circuit-breaker for repeated tool failures

- In `llmrunner/toolloop.go`, track consecutive failures per `(tool_name, path)`. After N
  identical failures (e.g., 2), inject a corrective system message and skip further calls to
  the same combination.

### Issue 4: `search==""` append-vs-replace ambiguity

- In `patch/search_replace.go`, when `search == ""` and the file already has content, replace
  (overwrite) instead of append. This matches the documented behavior and the tool description.

## Capabilities

### Modified Capabilities

- `search_replace` tool error diagnostics (more actionable messages)
- `RunToolLoop` circuit-breaker (prevent infinite loops)
- Prompt affected-file injection (inform LLM about new files)
- `ApplySearchReplace` overwrite semantics for empty search blocks

## Impact

| Area | Files Affected |
|------|----------------|
| Tool: search_replace | `server/internal/tool/tools/search_replace.go` |
| Patch Engine: search/replace apply | `server/internal/orchestrator/patch/search_replace.go` |
| LLM Runner: affected file injection | `server/internal/orchestrator/llmrunner/runner.go` |
| Tool Loop: circuit-breaker | `server/internal/orchestrator/llmrunner/toolloop.go` |
| Tests | `server/internal/tool/tools/search_replace_test.go` (new/updated) |
| Tests | `server/internal/orchestrator/patch/search_replace_test.go` (updated) |
| Tests | `server/internal/orchestrator/llmrunner/toolloop_test.go` (updated) |
| Tests | `server/internal/orchestrator/llmrunner/runner_test.go` (updated) |
