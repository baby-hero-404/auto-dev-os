# Tasks: Memory Hardening

> 4 issue độc lập — làm theo thứ tự value/effort: 2 → 3 → 1 → 4.

## 1. Secret patterns (REQ-002) — 0.5d

- [ ] 1.1 Extract `secretPatterns` ra `server/pkg/redact/redact.go` (giữ API `Redact(string) string`)
- [ ] 1.2 Thêm 7 pattern mới (AWS/Google/JWT/npm/gh family/PAT/Bearer)
- [ ] 1.3 Test corpus: mỗi loại ≥1 mẫu bị redact + ≥1 mẫu gần giống KHÔNG bị redact (false-positive guard)
- [ ] 1.4 `memory.go` dùng pkg mới; grep caller khác của patterns cũ

## 2. Circuit breaker (REQ-003, REQ-M01) — 1d

- [ ] 2.1 `server/pkg/llm/breaker.go`: closed/open/half-open + tests (threshold, cooldown, half-open recovery)
- [ ] 2.2 Bọc `MemoryEmbedder` bằng `BreakerEmbedder`; export `ErrEmbedderUnavailable`
- [ ] 2.3 `memory_search.go`: skip vector stream khi unavailable; test search vẫn trả kết quả 2-stream
- [ ] 2.4 `RecordObservation`: lưu với embedding NULL + `needs_embedding` flag + migration cột nếu cần

## 3. Decay sweep (REQ-001) — 2d

- [ ] 3.1 `repository/memory.go`: `PurgeExpired(floor)` loại trừ tier procedural + test
- [ ] 3.2 `StartDecaySweeper` ticker + wire vào server startup; interval/floor qua config
- [ ] 3.3 Backfill embeddings trong sweep tick (batch 100, chỉ khi breaker đóng)
- [ ] 3.4 Tests: sweep gọi ApplyDecay+Purge, lỗi không crash, backfill batch

## 4. MMR (REQ-004) — 1d

- [ ] 4.1 `mmrSelect(candidates, n, lambda)` + cosine helper trong `memory_search.go`
- [ ] 4.2 Wire sau rrfMerge (top-2N → N); NULL vector coi similarity 0
- [ ] 4.3 Tests: near-dup bị loại, top-1 giữ nguyên, đủ N khi thiếu ứng viên

## 5. Wrap-up

- [ ] 5.1 Update specs.md status + ARCHITECTURE.md (redact pkg mới, sweeper)
