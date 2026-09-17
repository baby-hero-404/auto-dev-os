package service

import (
	"context"
	"log/slog"
	"time"
)

// StartCooldownWorker clears expired credential cooldowns on a ticker. It
// lived in internal/gateway until the LLM API layer was removed; it is
// credential-pool infrastructure, not gateway logic, and CLI credentials
// cool down for quota exactly the same way API keys used to.
func StartCooldownWorker(ctx context.Context, pool *CredentialPoolService, interval time.Duration) {
	if pool == nil {
		return
	}
	if interval == 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := pool.ClearExpiredCooldowns(ctx)
			if err != nil {
				slog.Warn("clear provider credential cooldowns", "error", err)
				continue
			}
			if count > 0 {
				slog.Info("provider credential cooldowns cleared", "count", count)
			}
		}
	}
}
