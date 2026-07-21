package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// StartPRSyncWorker periodically checks if PRs for tasks in 'pr_ready' or 'human_review' states
// have been merged on the remote Git provider. If so, it automatically updates task and job status.
func StartPRSyncWorker(ctx context.Context, o *Orchestrator, checkInterval time.Duration) {
	if checkInterval == 0 {
		checkInterval = 1 * time.Minute
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	slog.Info("PR sync worker started", "interval", checkInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("PR sync worker stopped")
			return
		case <-ticker.C:
			syncPRStatuses(ctx, o)
		}
	}
}

func syncPRStatuses(ctx context.Context, o *Orchestrator) {
	// 1. List tasks currently in 'pr_ready' or 'human_review'
	statuses := []string{models.TaskStatusPrReady, models.TaskStatusHumanReview}
	tasks, err := o.tasks.ListRecentByStatus(ctx, statuses, 100)
	if err != nil {
		slog.Error("failed to list tasks for PR sync", "error", err)
		return
	}

	for _, t := range tasks {
		if len(t.PRURLs) == 0 {
			continue
		}

		if o.gitOps == nil {
			continue
		}

		repos, err := o.repositories.ListByProjectID(ctx, t.ProjectID)
		if err != nil {
			slog.Error("failed to list repositories for project", "project_id", t.ProjectID, "error", err)
			continue
		}

		allMerged := true
		for _, prURL := range t.PRURLs {
			var matchRepo string
			for _, r := range repos {
				baseRepo := strings.TrimSuffix(r.URL, ".git")
				if strings.Contains(prURL, baseRepo) {
					matchRepo = r.URL
					break
				}
			}
			if matchRepo == "" {
				allMerged = false
				break
			}

			merged, err := o.gitOps.IsPullRequestMerged(ctx, matchRepo, prURL)
			if err != nil {
				slog.Error("failed to check if PR is merged", "pr_url", prURL, "error", err)
				allMerged = false
				break
			}
			if !merged {
				allMerged = false
				break
			}
		}

		if allMerged {
			o.log(ctx, t.ID, nil, "info", "detected pull request has been merged remotely. transitioning task status to merged.")
			updated, err := o.updateTaskStatus(ctx, t.ID, models.TaskStatusMerged)
			if err != nil {
				slog.Error("failed to update task status to merged", "task_id", t.ID, "error", err)
				continue
			}

			// Clean up active/paused jobs
			if job, err := o.workflows.LatestByTaskID(ctx, t.ID); err == nil && job != nil && job.Status == models.WorkflowJobStatusPaused {
				_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
					"status":     models.WorkflowJobStatusDone,
					"step":       models.WorkflowStepDone,
					"last_error": "",
				})
				_ = o.checkpoint(ctx, t.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone})
			}
			slog.Info("task automatically updated to merged status", "task_id", updated.ID)
		}
	}
}

func (o *Orchestrator) SyncPRMerged(ctx context.Context, prURL string) (*models.Task, error) {
	// Find the task associated with the PR URL
	statuses := []string{models.TaskStatusPrReady, models.TaskStatusHumanReview}
	tasks, err := o.tasks.ListRecentByStatus(ctx, statuses, 100)
	if err != nil {
		return nil, err
	}

	var matchedTask *models.Task
	for i := range tasks {
		for _, url := range tasks[i].PRURLs {
			if url == prURL {
				matchedTask = &tasks[i]
				break
			}
		}
		if matchedTask != nil {
			break
		}
	}

	if matchedTask == nil {
		return nil, fmt.Errorf("no task found for PR URL: %s", prURL)
	}

	o.log(ctx, matchedTask.ID, nil, "info", "detected pull request has been merged remotely. transitioning task status to merged.")
	updated, err := o.updateTaskStatus(ctx, matchedTask.ID, models.TaskStatusMerged)
	if err != nil {
		return nil, err
	}

	// Clean up active/paused jobs
	if job, err := o.workflows.LatestByTaskID(ctx, matchedTask.ID); err == nil && job != nil && job.Status == models.WorkflowJobStatusPaused {
		_, _ = o.workflows.UpdateJob(ctx, job.ID, map[string]any{
			"status":     models.WorkflowJobStatusDone,
			"step":       models.WorkflowStepDone,
			"last_error": "",
		})
		_ = o.checkpoint(ctx, matchedTask.ID, &job.ID, models.WorkflowStepDone, map[string]any{"status": models.WorkflowJobStatusDone})
	}
	slog.Info("task automatically updated to merged status", "task_id", updated.ID)
	return updated, nil
}
