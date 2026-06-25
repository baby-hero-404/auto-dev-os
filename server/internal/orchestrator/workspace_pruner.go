package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// StartWorkspacePruner runs a periodic cleanup loop for old workspaces.
func (o *Orchestrator) StartWorkspacePruner(ctx context.Context) {
	if o.retention.Retention <= 0 {
		return
	}
	interval := o.retention.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	if removed, err := o.pruneWorkspaces(ctx); err != nil {
		observability.Warn(ctx, "workspace prune failed", "error", err)
	} else if removed > 0 {
		observability.Info(ctx, "workspace prune completed", "removed", removed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if removed, err := o.pruneWorkspaces(ctx); err != nil {
				observability.Warn(ctx, "workspace prune failed", "error", err)
			} else if removed > 0 {
				observability.Info(ctx, "workspace prune completed", "removed", removed)
			}
		}
	}
}

func (o *Orchestrator) pruneWorkspaces(ctx context.Context) (int, error) {
	root := o.workspaceRoot
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-o.retention.Retention)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "workspace prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		taskID := entry.Name()
		if o.tasks != nil {
			task, err := o.tasks.GetByID(ctx, taskID)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "record not found") {
					if err := o.RemoveWorkspace(taskID); err == nil {
						removed++
					}
				}
				continue
			}
			if task.Status == models.TaskStatusMerged || task.Status == models.TaskStatusFailed {
				if err := o.partialCleanupWorkspace(ctx, taskID); err != nil {
					observability.Warn(ctx, "workspace prune failed", "name", taskID, "error", err)
					continue
				}
				removed++
			}
		} else {
			if info.ModTime().Before(cutoff) {
				if err := o.RemoveWorkspace(entry.Name()); err != nil {
					observability.Warn(ctx, "workspace prune failed", "name", entry.Name(), "error", err)
					continue
				}
				removed++
			}
		}
	}
	return removed, nil
}

// StartLogPruner runs a periodic cleanup loop for old log files.
func (o *Orchestrator) StartLogPruner(ctx context.Context, retentionDays int, fileRoot string) {
	if retentionDays <= 0 || fileRoot == "" {
		return
	}
	interval := time.Hour
	if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
		observability.Warn(ctx, "log files prune failed", "error", err)
	} else if pruned > 0 {
		observability.Info(ctx, "log files prune completed", "pruned", pruned)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if pruned, err := pruneLogFiles(ctx, retentionDays, fileRoot); err != nil {
				observability.Warn(ctx, "log files prune failed", "error", err)
			} else if pruned > 0 {
				observability.Info(ctx, "log files prune completed", "pruned", pruned)
			}
		}
	}
}

func pruneLogFiles(ctx context.Context, retentionDays int, fileRoot string) (int, error) {
	entries, err := os.ReadDir(fileRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	pruned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			observability.Warn(ctx, "log prune stat failed", "name", entry.Name(), "error", err)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		filePath := filepath.Join(fileRoot, entry.Name())
		if err := os.Remove(filePath); err != nil {
			observability.Warn(ctx, "log prune remove failed", "path", filePath, "error", err)
			continue
		}
		pruned++
	}
	return pruned, nil
}
