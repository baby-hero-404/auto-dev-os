# Implementation Notes: Search-Replace Fuzzy Fallback

Spec: `docs/openspecs/search-replace-fuzzy-fallback/`.

## Starting state

Most of this spec's design was already implemented under a prior, undocumented pass: `server/internal/orchestrator/patch/search_replace_fuzzy.go` already had `relativeIndentMatch` (tier 2), `trimmedLineMatch` (a full per-line-trim tier, functionally the spec's tier 3 "line-trim"), `reindentReplace`/`indentDeltas`/`detectIndentChar`, and `nearestSimilarRange` (the REQ-M01 closest-match hint). What was missing: a distinct tier 1 "trailing-whitespace-only" matcher (REQ-002), correct tier ordering (the pre-existing code tried the full-trim tier before the relative-indent tier, skipping a trailing-whitespace-only tier entirely), and telemetry logging (REQ-006).

## What changed

1. **Added `trailingWSMatch`** (`search_replace_fuzzy.go`) as tier 1: strips only trailing `" \t"` per line (leading indent must match exactly), structurally mirroring `trimmedLineMatch`'s window-matching/ambiguity-detection logic.
2. **Reordered the fallback pipeline** in `ApplySearchReplace` (`search_replace.go`) to `trailingWSMatch (1) → relativeIndentMatch (2) → trimmedLineMatch (3)`, matching design.md's specified permissiveness ordering.
3. **Changed all three matcher signatures** from `(..., ok bool, ambiguous bool)` to `(..., ok bool, matchCount int)` so the ambiguous-match error (REQ-005) can report the exact number of candidates found, not just "multiple" — `fmt.Errorf("ambiguous match in %s (%s fallback found %d candidates)", relPath, tierNames[m.tier], matchCount)`.
4. **Added telemetry**: `log.Printf("search_replace tier=%d (%s) file=%s", ...)` on any successful fuzzy-tier match (REQ-006).

## Deviations from tasks.md

- **1.1/1.9 (corpus fixtures from `server/.data/logs/*.jsonl`)**: skipped. This environment's `.data/logs/` contains only 3 log files, none with a "search block not found" or "ambiguous match" entry — there is no real patch-fail corpus to harvest here. Table-driven synthetic tests were added instead (`TestTrailingWSMatch` in `search_replace_fuzzy_test.go`, `TestApplySearchReplace_AmbiguousFuzzyFallbackNamesTier` in `search_replace_test.go`). Revisit if/when production logs with real patch failures become available.
- **1.2 (`patch/fuzzy.go` as a separate file)**: kept the pre-existing `search_replace_fuzzy.go` file name/location rather than renaming, and the tier-dispatch loop (`findMatch`-equivalent) lives inline in `ApplySearchReplace` rather than as a standalone function, since the loop needs to splice the matched-and-reindented content back into `content` immediately per tier.

## Key files

- `server/internal/orchestrator/patch/search_replace_fuzzy.go` — the 3 fuzzy matchers, reindent/indent-delta helpers, `nearestSimilarRange`.
- `server/internal/orchestrator/patch/search_replace.go` — `ApplySearchReplace`'s tier-dispatch loop.
- `server/internal/orchestrator/patch/search_replace_fuzzy_test.go`, `search_replace_test.go` — per-matcher and end-to-end tests.
