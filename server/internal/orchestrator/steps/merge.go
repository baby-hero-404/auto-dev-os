package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// MergeStep implements Step for the merge phase.
type MergeStep struct {
	rt            StepRuntime
	tasks         TaskReader
	worktree      WorktreeManager
	workspace     WorkspaceLoader
	git           SandboxGitClient
	diff          DiffCapturer
	artifacts     ArtifactSaver
	status        StatusUpdater
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string
}

func NewMergeStep(
	rt StepRuntime,
	tasks TaskReader,
	worktree WorktreeManager,
	workspace WorkspaceLoader,
	git SandboxGitClient,
	diff DiffCapturer,
	artifacts ArtifactSaver,
	status StatusUpdater,
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
) *MergeStep {
	return &MergeStep{
		rt:            rt,
		tasks:         tasks,
		worktree:      worktree,
		workspace:     workspace,
		git:           git,
		diff:          diff,
		artifacts:     artifacts,
		status:        status,
		containerPath: containerPath,
	}
}

func (s *MergeStep) ID() string                         { return workflow.StepMerge }
func (s *MergeStep) StatusOnResume(_ StepResult) string { return models.TaskStatusReviewing }

func (s *MergeStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		if s.workspace != nil {
			if ws, errWS := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task); errWS == nil {
				for i := range ws.Repos {
					ws.Repos[i].Status.MergeStatus = models.MergeStatusSkipped
				}
				_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
			}
		}
		return StepResult{"status": "skipped", "info": "skipped merge step for easy task"}, nil
	}

	var targetRepos []models.Repository
	if s.worktree != nil {
		var errRepo error
		targetRepos, errRepo = s.worktree.LoadTargetRepositories(ctx, s.rt.Task)
		if errRepo != nil {
			targetRepos = nil
		}
	}

	var workspace *models.TaskWorkspace
	var ws *models.TaskWorkspace
	var errWS error
	if s.workspace != nil {
		ws, errWS = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
		if errWS == nil {
			workspace = ws
		}
	}

	integrationBranch := fmt.Sprintf("feature/%s", s.rt.Task.ID)
	beBranch := fmt.Sprintf("feature/%s-be", s.rt.Task.ID)
	feBranch := fmt.Sprintf("feature/%s-fe", s.rt.Task.ID)

	hasConflicts := false
	var conflictDetails []string

	for _, repo := range targetRepos {
		var localPath string
		if s.worktree != nil {
			localPath = s.worktree.RepoHostPath(s.rt.Task, workspace, repo)
		}
		containerLocalPath := s.containerPath(s.rt.Task, localPath, "")

		repoMergeStatus := models.MergeStatusMerged
		repoHasConflict := false

		// 1. Checkout integration branch
		errCheckout := s.git.CheckoutBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, integrationBranch)
		if errCheckout != nil {
			if s.workspace != nil && errWS == nil {
				for i := range ws.Repos {
					if ws.Repos[i].RepoID == repo.ID {
						ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
					}
				}
				_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
			}
			return nil, fmt.Errorf("checkout integration branch failed for repo %s: %w", repo.URL, errCheckout)
		}

		// Check if backend branch exists before merging
		hasBeBranch := s.git.HasBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, beBranch)
		if hasBeBranch {
			mergeStatus, errMergeBe := s.git.MergeBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, beBranch)
			if errMergeBe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (backend):\n%s", repo.URL, errMergeBe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if s.workspace != nil && errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
					}
					return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
				}
			}
		}

		// Check if frontend branch exists before merging (only if no conflict has occurred on this repo yet)
		hasFeBranch := false
		if !repoHasConflict {
			hasFeBranch = s.git.HasBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, feBranch)
		}
		if !repoHasConflict && hasFeBranch {
			mergeStatus, errMergeFe := s.git.MergeBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, feBranch)
			if errMergeFe != nil {
				if mergeStatus == models.MergeStatusConflict {
					hasConflicts = true
					repoHasConflict = true
					repoMergeStatus = models.MergeStatusConflict
					conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (frontend):\n%s", repo.URL, errMergeFe.Error()))
				} else {
					repoMergeStatus = models.MergeStatusFailed
					if s.workspace != nil && errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = repoMergeStatus
							}
						}
						_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
					}
					return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
				}
			}
		}

		if !repoHasConflict {
			// Commit the merge if there are staged changes
			commitMsg := "Merge role branches into integration"
			if errCommit := s.git.CommitChanges(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, commitMsg); errCommit != nil {
				if s.workspace != nil && errWS == nil {
					for i := range ws.Repos {
						if ws.Repos[i].RepoID == repo.ID {
							ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
						}
					}
					_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
				}
				return nil, fmt.Errorf("failed to commit merge for repo %s: %w", repo.URL, errCommit)
			}
		}

		if s.workspace != nil && errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					ws.Repos[i].Status.MergeStatus = repoMergeStatus
				}
			}
		}
	}

	if s.workspace != nil && errWS == nil {
		_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
	}

	if hasConflicts {
		conflictStr := strings.Join(conflictDetails, "\n")
		_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepMerge, "conflict", conflictStr)
		return nil, workflow.PauseError{
			Step:   workflow.StepMerge,
			Reason: fmt.Sprintf("merge conflict in files:\n%s\n— manual resolution required", conflictStr),
		}
	}

	var diffText string
	if s.diff != nil {
		var errDiff error
		diffText, errDiff = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepMerge, "")
		if errDiff != nil {
			return nil, fmt.Errorf("merge check failed: %w", errDiff)
		}
	}
	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusReviewing); err != nil {
			return nil, err
		}
	}
	return StepResult{
		"status":    "changes_reconciled",
		"info":      "local changes reconciled",
		"diff_size": len(diffText),
	}, nil
}
