package service

import (
	"sync"
	"time"
)

// embedderCircuitBreaker guards MemoryEmbedder.Embed() calls. After enough
// consecutive failures it "opens" and short-circuits further embed calls for
// a cooldown period, so a degraded/dead embedding provider doesn't add
// per-call timeout latency to every memory write/search — callers fall back
// to BM25-only search/storage-without-vector instead.
type embedderCircuitBreaker struct {
	mu               sync.Mutex
	failureThreshold int
	cooldown         time.Duration

	consecutiveFails int
	open             bool
	openedAt         time.Time
}

func newEmbedderCircuitBreaker(failureThreshold int, cooldown time.Duration) *embedderCircuitBreaker {
	return &embedderCircuitBreaker{failureThreshold: failureThreshold, cooldown: cooldown}
}

// Allow reports whether an embed call should be attempted right now. Once the
// cooldown elapses it allows a single trial call (half-open) to probe recovery.
func (b *embedderCircuitBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.open {
		return true
	}
	if time.Since(b.openedAt) >= b.cooldown {
		return true // half-open trial
	}
	return false
}

func (b *embedderCircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails = 0
	b.open = false
}

func (b *embedderCircuitBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails++
	if b.consecutiveFails >= b.failureThreshold {
		b.open = true
		b.openedAt = time.Now()
	}
}
