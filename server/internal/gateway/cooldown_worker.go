package gateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
)

func StartCooldownWorker(ctx context.Context, pool *service.CredentialPoolService, interval time.Duration) {
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
