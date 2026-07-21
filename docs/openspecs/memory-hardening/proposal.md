# Proposal: Memory Hardening (P0.2–P0.4 + P3.4)

## Why

Hệ memory 4-tier + RRF hybrid search đã tồn tại và tốt (verified 2026-07-20), nhưng có 4 gap cụ thể đã xác minh trực tiếp trên source (`docs/references/README.md` §Confirmed Gaps — Memory):

1. `MemoryService.ApplyDecay()` (`server/internal/service/memory.go:136`) **không có caller nào** — decay logic viết rồi nhưng chưa bao giờ chạy.
2. `secretPatterns` (`memory.go:212-217`) chỉ 5 regex — thiếu AWS `AKIA`, Google `AIza`, JWT, `npm_`, GitHub `ghs_/ghu_/github_pat_`. Memory ghi raw tool output nên đây là leak surface thật.
3. `MemoryEmbedder.Embed()` (`server/pkg/llm/embedding.go`) là plain HTTP call không circuit breaker — embedding provider chết làm `RecordObservation`/`Search` fail toàn phần thay vì degrade về BM25-only.
4. Không có MMR/diversity dedup sau `rrfMerge` (`memory_search.go:78`) — top-N có thể là near-duplicates, phí context tokens.

## What Changes

### Issue 1: Wire decay sweep
- Ticker định kỳ (mặc định 6h, configurable) gọi `ApplyDecay()` — theo pattern có sẵn ở `orchestrator/cache_workers.go:19`.
- Nâng decay function: thêm TTL sweep (xóa/archive memory có `decay_score` dưới ngưỡng) ngoài phép nhân `*= 0.95` hiện có.

### Issue 2: Secret patterns mở rộng
- Bổ sung regexes: AWS access key, Google API key, JWT (3-segment base64url), npm token, GitHub token họ mới, generic `Bearer` header. Kèm test corpus.

### Issue 3: Embedder circuit breaker
- Bọc `Embed()` bằng circuit breaker (threshold N lỗi liên tiếp → open trong M phút). Khi open: `RecordObservation` lưu không có vector (backfill sau), `Search` chạy BM25+graph, bỏ nhánh vector.

### Issue 4: MMR diversity
- Sau `rrfMerge`, áp MMR (λ≈0.7) trên top-2N để chọn N kết quả đa dạng theo cosine similarity của embeddings đã có.

## Capabilities

### New Capabilities
- Decay sweep tự động chạy nền; TTL cleanup.
- Circuit breaker + graceful degradation cho embedding.
- MMR dedup trên search results.

### Modified Capabilities
- Secret redaction coverage rộng hơn.
- `Search` trả kết quả đa dạng hơn với cùng N.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Service | `server/internal/service/memory.go`, `memory_search.go` |
| Repository | `server/internal/repository/memory.go` (TTL sweep query) |
| LLM pkg | `server/pkg/llm/embedding.go` (breaker wrapper) |
| Workers | worker/ticker mới theo pattern `orchestrator/cache_workers.go` |
