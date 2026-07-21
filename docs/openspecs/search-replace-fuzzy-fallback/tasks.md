# Tasks: Search-Replace Fuzzy Fallback

- [ ] 1.1 Thu corpus: grep patch-fail thật từ `server/.data/logs/*.jsonl` → fixtures trong `patch/testdata/fuzzy/`
- [ ] 1.2 `patch/fuzzy.go`: khung `findMatch` + tierNames + line-index infrastructure
- [ ] 1.3 `exactMatch` (port hành vi cũ) — test suite cũ pass nguyên vẹn (REQ-001)
- [ ] 1.4 `trailingWSMatch` + tests (REQ-002)
- [ ] 1.5 `relativeIndentMatch` + re-indent replace + tests (REQ-003)
- [ ] 1.6 `lineTrimMatch` + indent preservation + tests (REQ-004)
- [ ] 1.7 Multi-match fail-fast per tier + tests (REQ-005)
- [ ] 1.8 `notFoundWithHint` error + tier telemetry log (REQ-006, REQ-M01)
- [ ] 1.9 Chạy corpus fixtures — ghi tỷ lệ patch được cứu vào design.md
- [ ] 1.10 Update specs.md status
