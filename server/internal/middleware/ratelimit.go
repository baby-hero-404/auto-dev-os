package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a per-key token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens added per interval
	burst    int           // max tokens per bucket
	interval time.Duration // refill interval
}

type bucket struct {
	tokens   int
	lastFill time.Time
}

// NewRateLimiter creates a rate limiter. rate is tokens per interval, burst is
// the maximum tokens a single bucket can hold (allows small bursts).
func NewRateLimiter(rate, burst int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		burst:    burst,
		interval: interval,
	}
	// Periodically evict stale buckets to prevent memory leaks.
	go rl.cleanup()
	return rl
}

// Allow returns true if the key has remaining tokens.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	now := time.Now()
	if !ok {
		rl.buckets[key] = &bucket{tokens: rl.burst - 1, lastFill: now}
		return true
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastFill)
	refill := int(elapsed / rl.interval) * rl.rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > rl.burst {
			b.tokens = rl.burst
		}
		b.lastFill = now
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for k, b := range rl.buckets {
			if b.lastFill.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns an HTTP middleware that limits requests per user (or IP for
// unauthenticated requests).
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use user ID from JWT claims if available, otherwise use remote IP.
			key := r.RemoteAddr
			if claims, ok := r.Context().Value(authClaimsKey).(*tokenClaims); ok && claims != nil {
				key = claims.Subject
			}
			if !limiter.Allow(key) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
