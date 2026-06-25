package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteMerge(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := deps.Tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		if deps.Wkspace != nil {
			if ws, errWS := deps.Wkspace.LoadTaskWorkspace(ctx, task); errWS == nil {
				for i := range ws.Repos {
					ws.Repos[i].Status.MergeStatus = models.MergeStatusSkipped
				}
				_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
			}
		}
		return map[string]any{"status": "skipped", "info": "skipped merge step for easy task"}, nil
	}

	var targetRepos []models.Repository
	if deps.RepoUtil != nil {
		var errRepo error
		targetRepos, errRepo = deps.RepoUtil.LoadTargetRepositories(ctx, task)
		if errRepo != nil {
			targetRepos = nil
		}
	}

	var workspace *models.TaskWorkspace
	var ws *models.TaskWorkspace
	var errWS error
	if deps.Wkspace != nil {
		ws, errWS = deps.Wkspace.LoadTaskWorkspace(ctx, task)
		if errWS == nil {
			workspace = ws
		}
	}

	integrationBranch := fmt.Sprintf("feature/%s", task.ID)
	beBranch := fmt.Sprintf("feature/%s-be", task.ID)
	feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

	hasConflicts := false
	var conflictDetails []string

	for _, repo := range targetRepos {
		var localPath string
		if deps.RepoUtil != nil {
			localPath = deps.RepoUtil.RepoHostPath(task, workspace, repo)
		}
		containerLocalPath := deps.ContainerPathForHostPath(task, localPath, "")

		repoMergeStatus := models.MergeStatusMerged
		repoHasConflict := false

		// 1. Checkout integration branch
		errCheckout := deps.SandboxGit.CheckoutBranch(ctx, task, agent, containerLocalPath, integrationBranch)
		if errCheckout != nil {
			if deps.Wkspace != nil && errWS == nil {
				for i := range ws.Repos {
					if ws.Repos[i].RepoID == repo.ID {
						ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
					}
				}
				_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
			}
			return nil, fmt.Errorf("checkout integration branch failed for repo %s: %w", repo.URL, errCheckout)
		}

		// Check if backend branch exists before merging
		hasBeBranch := deps.SandboxGit.HasBranch(ctx, task, agent, containerLocalPath, beBranch)
		if hasBeBranch {
			mergeStatus, errMergeBe := deps.SandboxGit.MergeBranch(ctx, task, agent, containerLocalPath, beBranch)
			if errMergeBe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (backend):\n%s", repo.URL, errMergeBe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if deps.Wkspace != nil && errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
					}
					return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
				}
			}
		}

		// Check if frontend branch exists before merging (only if no conflict has occurred on this repo yet)
		hasFeBranch := false
		if !repoHasConflict {
			hasFeBranch = deps.SandboxGit.HasBranch(ctx, task, agent, containerLocalPath, feBranch)
		}
		if !repoHasConflict && hasFeBranch {
			mergeStatus, errMergeFe := deps.SandboxGit.MergeBranch(ctx, task, agent, containerLocalPath, feBranch)
			if errMergeFe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (frontend):\n%s", repo.URL, errMergeFe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if deps.Wkspace != nil && errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
					}
					return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
				}
			}
		}

		if !repoHasConflict {
			// Commit the merge if there are staged changes
			commitMsg := "Merge role branches into integration"
			if errCommit := deps.SandboxGit.CommitChanges(ctx, task, agent, containerLocalPath, commitMsg); errCommit != nil {
				if deps.Wkspace != nil && errWS == nil {
					for i := range ws.Repos {
						if ws.Repos[i].RepoID == repo.ID {
							ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
						}
					}
					_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
				}
				return nil, fmt.Errorf("failed to commit merge for repo %s: %w", repo.URL, errCommit)
			}
		}

		if deps.Wkspace != nil && errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					ws.Repos[i].Status.MergeStatus = repoMergeStatus
				}
			}
		}
	}

	if deps.Wkspace != nil && errWS == nil {
		_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
	}

	if hasConflicts {
		conflictStr := strings.Join(conflictDetails, "\n")
		_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepMerge, "conflict", conflictStr)
		return nil, workflow.PauseError{
			Step:   workflow.StepMerge,
			Reason: fmt.Sprintf("merge conflict in files:\n%s\n— manual resolution required", conflictStr),
		}
	}

	var diffText string
	if deps.RepoUtil != nil {
		var errDiff error
		diffText, errDiff = deps.RepoUtil.CaptureWorkspaceDiff(ctx, task, agent, workflow.StepMerge, "")
		if errDiff != nil {
			return nil, fmt.Errorf("merge check failed: %w", errDiff)
		}
	}
	if _, err := deps.UpdateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":    "changes_reconciled",
		"info":      "local changes reconciled",
		"diff_size": len(diffText),
	}, nil
}
