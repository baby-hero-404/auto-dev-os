# Design: Memory Hardening

## 1. Decay sweep worker

Copy pattern `orchestrator/cache_workers.go:19` (ticker + ctx cancel). Đặt tại service layer (không phụ thuộc orchestrator):

```go
func (s *MemoryService) StartDecaySweeper(ctx context.Context, interval time.Duration) {
    go func() {
        t := time.NewTicker(interval); defer t.Stop()
        for { select {
        case <-ctx.Done(): return
        case <-t.C:
            if err := s.ApplyDecay(ctx); err != nil { s.log.Error("decay sweep", "err", err) }
            if err := s.repo.PurgeExpired(ctx, s.cfg.DecayFloor); err != nil { ... }
        }}
    }()
}
```

- `PurgeExpired`: `DELETE FROM memories WHERE decay_score < $floor AND tier != 'procedural'` (procedural = learned skills, không tự xóa). Floor default 0.05, config qua env.
- Wire trong server startup cùng chỗ start các worker khác.

## 2. Secret patterns

Bổ sung vào `secretPatterns` (`memory.go:212`):

```go
`AKIA[0-9A-Z]{16}`                                  // AWS access key
`AIza[0-9A-Za-z\-_]{35}`                            // Google API key
`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` // JWT
`npm_[A-Za-z0-9]{36}`                               // npm token
`gh[psuor]_[A-Za-z0-9]{36,}`                        // GitHub token family (gộp ghp_ hiện có)
`github_pat_[A-Za-z0-9_]{22,}`                      // GitHub fine-grained PAT
`(?i)bearer\s+[A-Za-z0-9\-._~+/]{20,}=*`            // Authorization header
```

Cân nhắc extract patterns ra `pkg/redact/` để CLI engine set (`pluggable-execution-engine` task 3.4) dùng chung — làm luôn trong set này.

## 3. Circuit breaker

Không thêm dependency: breaker ~60 dòng tự viết (states closed/open/half-open, atomic counter, mutex) trong `pkg/llm/breaker.go`, bọc `MemoryEmbedder`:

```go
type BreakerEmbedder struct { inner Embedder; br *Breaker }
func (b *BreakerEmbedder) Embed(ctx, text) ([]float32, error) {
    if !b.br.Allow() { return nil, ErrEmbedderUnavailable }
    v, err := b.inner.Embed(ctx, text)
    b.br.Record(err == nil)
    return v, err
}
```

- `memory_search.go`: khi `errors.Is(err, ErrEmbedderUnavailable)` → skip vector stream, tiếp tục BM25+graph (RRF với 2 streams).
- `RecordObservation`: lưu row với `embedding = NULL`, cột/flag `needs_embedding = true`; decay sweeper tick cũng backfill batch nhỏ (limit 100/tick) khi breaker đóng.

## 4. MMR

Trong `memory_search.go` sau `rrfMerge`:

```
candidates = rrfMerge(...)[:2N]
selected = [candidates[0]]
while len(selected) < N:
    pick argmax over remaining: λ*rrfScore(c) - (1-λ)*max cosine(c, s∈selected)
```

- Chỉ chạy khi embeddings có sẵn trên candidates (rows vector NULL → coi similarity 0, luôn "đa dạng").
- λ=0.7 hằng số, không cần config ngay.

## Trade-offs

- Tự viết breaker thay vì lib (sony/gobreaker): tránh dependency cho 1 use-case; nếu sau này cần breaker ở chỗ khác thì cân nhắc lib.
- Purge là DELETE cứng (trừ procedural): memory là dữ liệu derive được, không cần soft-delete phức tạp.
