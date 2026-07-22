# Implementation Notes: Tool-Output Filtering Pipeline

Spec: `docs/openspecs/tool-output-filtering/`.

## Starting state

The only content-shaping applied to tool output was `truncateToolResult` (`toolloop.go`), a flat hard-cut at `maxToolResultChars` (8000) with no awareness of repeated lines, ANSI noise, or error signal location.

## What was added

New package `server/internal/orchestrator/llmrunner/outputfilter/`:

- `filter.go` — `Profile` (flags: StripANSI/Dedup/PathCompress/ErrorPriority), a name-keyed `toolProfiles` registry (`run_build`/`run_lint` → build, `run_tests` → test, `git_diff`/`git_status` → diff, `read_file` → read, everything else → default), and `Run(toolName, output, budget) (string, Stats)` which dispatches the pipeline in design.md's order (strip → dedup → pathcompress → errorpriority).
- `strip.go` — ANSI escape-code stripping + `\r` progress-bar collapse (keeps only the text after the last `\r` on a line).
- `dedup.go` — collapses runs of ≥3 identical consecutive lines into the line + `[repeated N times]`.
- `errorpriority.go` — `errorPriorityTruncate` keeps every line matching the error regex plus 2 lines of context each side, merged with the first/last 20 lines, replacing gaps with `[... M lines omitted ...]`; `tailCutIfNeeded` is the `diff` profile's simpler tail-cut-with-marker fallback.
- `pathcompress.go` — see deviation below.

`toolloop.go`: `outputfilter.Run(call.Name, result, maxToolResultChars)` is called immediately before the existing `truncateToolResult`, which remains completely unchanged as the final safety net (REQ-005). `slog.Info("outputfilter", "tool", ..., "in", ..., "out", ..., "saved_pct", ...)` logs whenever input ≥ 1KB (REQ-006).

## Deviations from tasks.md / design.md

- **`pathcompress.go` is a no-op** (task 1.5): rewriting a repeated absolute path to a relative one after its first occurrence would change the bytes of a kept line. REQ-007 is explicit that a filter may only delete/merge/mark lines, never rewrite their content, and the line-subsequence property test enforces this. Implementing real path compression as specified would fail that same test. It's kept as a wired-but-inert pipeline stage (still invoked for `build`/`test` profiles) so a content-safe version — e.g. compressing paths only inside a synthesized marker line, never inside an original line — can be dropped in later without touching `Run`'s call sites.
- **Per-tool profile declared via a name-keyed registry, not per-tool metadata** (task 1.7): rather than adding an `OutputProfile` field to every tool definition file under `internal/tool/tools/*.go` (6+ files touched for a value that's 1:1 with the tool's already-unique registered name), `filter.go`'s `toolProfiles map[string]Profile` does the same job. `ProfileFor(toolName)` is the single lookup point; REQ-004's scenarios (git_diff skips dedup, unknown tool gets default) are satisfied identically.
- **No real fixture corpus** (task 1.1/1.10): `server/.data/logs/*.jsonl` had no usable build/test/diff/ANSI samples (same gap noted in `search-replace-fuzzy-fallback`), so tests use synthetic fixtures constructed in `filter_test.go` instead of golden files loaded from disk.

## Measured savings (illustrative, not from a real corpus)

A synthetic 401-line build log (200 lines of ANSI-colored "ok" noise, 1 FATAL line, 200 more noise lines, ~10KB) filtered through the `build` profile at an 2000-char budget: **in=10040 out=105 bytes, saved≈99%** — the FATAL line and head/tail context survive, the bulk of repeated/noise lines are omitted. This is a synthetic worst-case (fully repetitive log), not a representative production sample; real savings will vary with how noisy actual build/test output is.

## Key files

- `server/internal/orchestrator/llmrunner/outputfilter/filter.go` — `Profile`, `toolProfiles`, `ProfileFor`, `Run`, `Stats`.
- `server/internal/orchestrator/llmrunner/outputfilter/strip.go` — `stripANSI`.
- `server/internal/orchestrator/llmrunner/outputfilter/dedup.go` — `dedupLines`.
- `server/internal/orchestrator/llmrunner/outputfilter/errorpriority.go` — `errorPriorityTruncate`, `tailCutIfNeeded`.
- `server/internal/orchestrator/llmrunner/outputfilter/pathcompress.go` — no-op (see deviation).
- `server/internal/orchestrator/llmrunner/toolloop.go` — wiring before `truncateToolResult`.
- Tests: `server/internal/orchestrator/llmrunner/outputfilter/filter_test.go`.
