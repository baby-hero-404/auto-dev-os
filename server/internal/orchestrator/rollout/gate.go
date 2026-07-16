package rollout

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GateResult struct {
	TasksSampled      int            `json:"tasks_sampled"`
	TotalCalls        int            `json:"total_calls"`
	TotalViolations   int            `json:"total_violations"`
	ViolationRatePct  float64        `json:"violation_rate_pct"`
	Pass              bool           `json:"pass"`
	TopViolationTypes map[string]int `json:"top_violation_types"`
}

func EvaluateStateMachineGate(ctx context.Context, tasks *repository.TaskRepo, logs *repository.WorkflowRepo, sampleSize int, thresholdPct float64) (GateResult, error) {
	terminalStatuses := []string{
		models.TaskStatusMerged,
		models.TaskStatusPrReady,
		models.TaskStatusHumanReview,
		models.TaskStatusFailed,
	}

	taskList, err := tasks.ListRecentByStatus(ctx, terminalStatuses, sampleSize)
	if err != nil {
		return GateResult{}, fmt.Errorf("list recent terminal tasks: %w", err)
	}

	result := GateResult{
		TasksSampled:      len(taskList),
		TopViolationTypes: make(map[string]int),
	}

	if len(taskList) == 0 {
		result.Pass = true
		return result, nil
	}

	taskIDs := make([]string, len(taskList))
	for i, t := range taskList {
		taskIDs[i] = t.ID
	}

	// Count total calls (assembled prompt with...)
	totalCalls, err := logs.CountTotalCalls(ctx, taskIDs)
	if err != nil {
		return GateResult{}, fmt.Errorf("count total calls: %w", err)
	}
	result.TotalCalls = totalCalls

	// Find violation logs ([TELEMETRY-VIOLATION]...)
	violationLogs, err := logs.FindLogsByPattern(ctx, taskIDs, "warn", "[TELEMETRY-VIOLATION]")
	if err != nil {
		return GateResult{}, fmt.Errorf("find violation logs: %w", err)
	}
	result.TotalViolations = len(violationLogs)

	if totalCalls > 0 {
		result.ViolationRatePct = (float64(len(violationLogs)) / float64(totalCalls)) * 100.0
	}

	result.Pass = result.ViolationRatePct <= thresholdPct

	for _, log := range violationLogs {
		msg := log.Message
		var bucket string
		if strings.Contains(msg, "is not permitted") {
			bucket = "tool not permitted"
		} else if strings.Contains(msg, "out-of-scope write") {
			bucket = "out-of-scope write"
		} else if strings.Contains(msg, "write tool call detected during StateDiscovery") {
			bucket = "write in discovery"
		} else {
			bucket = "other violation"
		}
		result.TopViolationTypes[bucket]++
	}

	return result, nil
}
