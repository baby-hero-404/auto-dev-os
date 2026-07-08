package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
)

// StartAgentWatchdog runs a background loop to reset stuck agents (status: assigned/running but not updated for a long time) to idle.
func StartAgentWatchdog(ctx context.Context, repo *repository.AgentRepo, checkInterval time.Duration, stuckDuration time.Duration) {
	if repo == nil {
		return
	}
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute
	}
	if stuckDuration == 0 {
		stuckDuration = 30 * time.Minute
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-stuckDuration)
			stuckAgents, err := repo.ResetStuckAgents(ctx, cutoff)
			if err != nil {
				slog.Error("agent watchdog check failed", "error", err)
				continue
			}
			if len(stuckAgents) > 0 {
				for _, agent := range stuckAgents {
					slog.Warn("stuck agent recovered by watchdog",
						"agent_id", agent.ID,
						"name", agent.Name,
						"role", agent.Role,
						"last_updated", agent.UpdatedAt,
					)
				}
			}
		}
	}
}
