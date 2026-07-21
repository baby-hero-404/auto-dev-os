# Tasks: Tool-Output Filtering

- [ ] 1.1 Thu fixtures thật từ `server/.data/logs/` (build fail, test lặp, diff, ANSI) → `outputfilter/testdata/`
- [ ] 1.2 `outputfilter/filter.go`: khung Filter/Profile/Run + Stats
- [ ] 1.3 `strip.go` (ANSI + \r handling) + tests (REQ-003)
- [ ] 1.4 `dedup.go` (ngưỡng 3, marker) + tests (REQ-001)
- [ ] 1.5 `pathcompress.go` + tests
- [ ] 1.6 `errorpriority.go` (patterns, context merge, budget) + golden tests (REQ-002)
- [ ] 1.7 Profile registry + `OutputProfile` metadata trên tools (build/test/diff/read/default) (REQ-004)
- [ ] 1.8 Wire vào `toolloop.go` trước hard-cut + metrics log (REQ-005, REQ-006)
- [ ] 1.9 Property test: line-subsequence invariant (REQ-007)
- [ ] 1.10 Chạy trên fixtures, ghi savings % vào design.md; update specs.md status
