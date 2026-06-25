package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepMerge(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := o.tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		if ws, errWS := o.LoadTaskWorkspace(ctx, task); errWS == nil {
			for i := range ws.Repos {
				ws.Repos[i].Status.MergeStatus = models.MergeStatusSkipped
			}
			_ = o.SaveTaskWorkspaceMetadata(task, ws)
		}
		return map[string]any{"status": "skipped", "info": "skipped merge step for easy task"}, nil
	}

	targetRepos, err := o.loadTargetRepositories(ctx, task)
	if err != nil {
		targetRepos = nil
	}

	ws, errWS := o.LoadTaskWorkspace(ctx, task)
	var workspace *models.TaskWorkspace
	if errWS == nil {
		workspace = ws
	}

	integrationBranch := fmt.Sprintf("feature/%s", task.ID)
	beBranch := fmt.Sprintf("feature/%s-be", task.ID)
	feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

	hasConflicts := false
	var conflictDetails []string

	for _, repo := range targetRepos {
		localPath := o.repoHostPath(task, workspace, repo)
		containerLocalPath := o.containerPathForHostPath(task, localPath, "")

		repoMergeStatus := models.MergeStatusMerged
		repoHasConflict := false

		// 1. Checkout integration branch
		errCheckout := o.sandboxGit.CheckoutBranch(ctx, task, agent, containerLocalPath, integrationBranch)
		if errCheckout != nil {
			if errWS == nil {
				for i := range ws.Repos {
					if ws.Repos[i].RepoID == repo.ID {
						ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
					}
				}
				_ = o.SaveTaskWorkspaceMetadata(task, ws)
			}
			return nil, fmt.Errorf("checkout integration branch failed for repo %s: %w", repo.URL, errCheckout)
		}

		// Check if backend branch exists before merging
		hasBeBranch := o.sandboxGit.HasBranch(ctx, task, agent, containerLocalPath, beBranch)
		if hasBeBranch {
			mergeStatus, errMergeBe := o.sandboxGit.MergeBranch(ctx, task, agent, containerLocalPath, beBranch)
			if errMergeBe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (backend):\n%s", repo.URL, errMergeBe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = o.SaveTaskWorkspaceMetadata(task, ws)
					}
					return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
				}
			}
		}

		// Check if frontend branch exists before merging (only if no conflict has occurred on this repo yet)
		hasFeBranch := false
		if !repoHasConflict {
			hasFeBranch = o.sandboxGit.HasBranch(ctx, task, agent, containerLocalPath, feBranch)
		}
		if !repoHasConflict && hasFeBranch {
			mergeStatus, errMergeFe := o.sandboxGit.MergeBranch(ctx, task, agent, containerLocalPath, feBranch)
			if errMergeFe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (frontend):\n%s", repo.URL, errMergeFe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = o.SaveTaskWorkspaceMetadata(task, ws)
					}
					return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
				}
			}
		}

		if !repoHasConflict {
			// Commit the merge if there are staged changes
			commitMsg := "Merge role branches into integration"
			if errCommit := o.sandboxGit.CommitChanges(ctx, task, agent, containerLocalPath, commitMsg); errCommit != nil {
				if errWS == nil {
					for i := range ws.Repos {
						if ws.Repos[i].RepoID == repo.ID {
							ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
						}
					}
					_ = o.SaveTaskWorkspaceMetadata(task, ws)
				}
				return nil, fmt.Errorf("failed to commit merge for repo %s: %w", repo.URL, errCommit)
			}
		}

		if errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					ws.Repos[i].Status.MergeStatus = repoMergeStatus
				}
			}
		}
	}

	if errWS == nil {
		_ = o.SaveTaskWorkspaceMetadata(task, ws)
	}

	if hasConflicts {
		conflictStr := strings.Join(conflictDetails, "\n")
		_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepMerge, "conflict", conflictStr)
		return nil, workflow.PauseError{
			Step:   workflow.StepMerge,
			Reason: fmt.Sprintf("merge conflict in files:\n%s\n— manual resolution required", conflictStr),
		}
	}

	diffText, err := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepMerge, "")
	if err != nil {
		return nil, fmt.Errorf("merge check failed: %w", err)
	}
	if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":    "changes_reconciled",
		"info":      "local changes reconciled",
		"diff_size": len(diffText),
	}, nil
}
