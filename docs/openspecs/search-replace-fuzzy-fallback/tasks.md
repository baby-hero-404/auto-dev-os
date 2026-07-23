# Tasks: Search-Replace Fuzzy Fallback

- [x] 1.1 Thu corpus: grep patch-fail thật từ `server/.data/logs/*.jsonl` → fixtures trong `patch/testdata/fuzzy/` — skipped: no patch-fail entries exist in local `.data/logs/*.jsonl` (only 3 log files, none contain "search block not found"/"ambiguous match"); best-effort only per proposal, revisit when production logs are available.
- [x] 1.2 `patch/fuzzy.go`: khung `findMatch` + tierNames + line-index infrastructure — implemented as `search_replace_fuzzy.go` (pre-existing file, extended); dispatch loop lives inline in `ApplySearchReplace` (`search_replace.go`) rather than a separate `findMatch`, since the loop also needs to reindent/splice content per matched tier.
- [x] 1.3 `exactMatch` (port hành vi cũ) — test suite cũ pass nguyên vẹn (REQ-001) — unchanged: `strings.Count`/`strings.Replace` exact-match path in `ApplySearchReplace` kept byte-identical; all pre-existing tests pass unmodified.
- [x] 1.4 `trailingWSMatch` + tests (REQ-002) — added `trailingWSMatch` (tier 1) in `search_replace_fuzzy.go`; table-driven tests in `search_replace_fuzzy_test.go` (`TestTrailingWSMatch`).
- [x] 1.5 `relativeIndentMatch` + re-indent replace + tests (REQ-003) — pre-existing, reordered to tier 2 (was previously tried after `trimmedLineMatch`).
- [x] 1.6 `lineTrimMatch` + indent preservation + tests (REQ-004) — pre-existing `trimmedLineMatch`, reordered to tier 3 (last, most permissive).
- [x] 1.7 Multi-match fail-fast per tier + tests (REQ-005) — all 3 matchers return `matchCount int` (not just an `ambiguous bool`); `ApplySearchReplace` fails immediately with tier name + exact candidate count on `matchCount > 1`, no fallthrough to a fuzzier tier. Covered by `TestApplySearchReplace_AmbiguousFuzzyFallbackNamesTier` + per-matcher ambiguous-case tests.
- [x] 1.8 `notFoundWithHint` error + tier telemetry log (REQ-006, REQ-M01) — telemetry: `log.Printf("search_replace tier=%d (%s) file=%s", ...)` on successful fuzzy match. Hint: pre-existing `nearestSimilarRange` (token-overlap closest-match) reused unchanged for the final all-tiers-failed error.
- [x] 1.9 Chạy corpus fixtures — ghi tỷ lệ patch được cứu vào design.md — skipped, see 1.1 (no corpus available in this environment).
- [x] 1.10 Update specs.md status — done.

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
