package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type TaskService struct {
	repo        *repository.TaskRepo
	projectRepo *repository.ProjectRepo
	repoRepo    *repository.RepositoryRepo
}

func NewTaskService(repo *repository.TaskRepo, projectRepo *repository.ProjectRepo, repoRepo *repository.RepositoryRepo) *TaskService {
	return &TaskService{repo: repo, projectRepo: projectRepo, repoRepo: repoRepo}
}

func (s *TaskService) Create(ctx context.Context, projectID string, input models.CreateTaskInput) (*models.Task, error) {
	if input.Title == "" {
		return nil, ErrValidation("title is required")
	}
	if input.RepositoryID != nil && *input.RepositoryID != "" {
		repo, err := s.repoRepo.GetByID(ctx, *input.RepositoryID)
		if err != nil {
			return nil, ErrValidation("invalid repository_id")
		}
		if repo.ProjectID != projectID {
			return nil, ErrValidation("repository does not belong to the project")
		}
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *TaskService) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) ListByProjectID(ctx context.Context, projectID string) ([]models.Task, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *TaskService) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	// Enforce task lifecycle state machine.
	if input.Status != nil {
		task, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := workflow.ValidateTaskTransition(task.Status, *input.Status); err != nil {
			return nil, ErrValidation(err.Error())
		}
	}
	return s.repo.Update(ctx, id, input)
}

func (s *TaskService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *TaskService) Analyze(ctx context.Context, id string) (*models.Task, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	analysis := buildTaskAnalysis(task)
	raw, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	project, err := s.projectRepo.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	status := models.TaskStatusAnalyzing
	specStatus := models.TaskSpecStatusDraft
	if len(analysis.ClarificationQuestions) > 0 {
		specStatus = models.TaskSpecStatusChangesRequested
		status = models.TaskStatusSpecReview
	} else if project.AutoReviewPolicy == "always_review" {
		specStatus = models.TaskSpecStatusPendingReview
		status = models.TaskStatusSpecReview
	} else if analysis.Complexity == models.TaskComplexityEasy || project.AutoReviewPolicy == "auto_merge" {
		specStatus = models.TaskSpecStatusAutoApproved
		status = models.TaskStatusCoding
	} else {
		specStatus = models.TaskSpecStatusPendingReview
		status = models.TaskStatusSpecReview
	}
	if task.Status != models.TaskStatusTodo &&
		task.Status != models.TaskStatusAnalyzing &&
		task.Status != models.TaskStatusSpecReview &&
		task.Status != models.TaskStatusFailed &&
		task.Status != "" {
		return nil, ErrValidation(fmt.Sprintf("invalid task transition from %q to %q during analysis", task.Status, status))
	}

	return s.repo.Update(ctx, id, models.UpdateTaskInput{
		Status:     &status,
		Complexity: &analysis.Complexity,
		Analysis:   json.RawMessage(raw),
		SpecStatus: &specStatus,
	})
}

func (s *TaskService) Clarify(ctx context.Context, id string, input models.ClarifyTaskInput) (*models.Task, error) {
	if strings.TrimSpace(input.Context) == "" {
		return nil, ErrValidation("context is required")
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	description := strings.TrimSpace(task.Description + "\n\nClarification:\n" + input.Context)
	if _, err := s.repo.Update(ctx, id, models.UpdateTaskInput{Description: &description}); err != nil {
		return nil, err
	}
	return s.Analyze(ctx, id)
}

func (s *TaskService) GetAnalysis(ctx context.Context, id string) (json.RawMessage, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return task.Analysis, nil
}

func (s *TaskService) ApproveAnalysis(ctx context.Context, id string) (*models.Task, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	specStatus := models.TaskSpecStatusApproved
	status := models.TaskStatusCoding
	if err := workflow.ValidateTaskTransition(task.Status, status); err != nil {
		return nil, ErrValidation(err.Error())
	}
	return s.repo.Update(ctx, id, models.UpdateTaskInput{SpecStatus: &specStatus, Status: &status})
}

func (s *TaskService) RequestAnalysisChanges(ctx context.Context, id string, input models.ClarifyTaskInput) (*models.Task, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	specStatus := models.TaskSpecStatusChangesRequested
	status := models.TaskStatusSpecReview
	if err := workflow.ValidateTaskTransition(task.Status, status); err != nil {
		return nil, ErrValidation(err.Error())
	}
	description := task.Description
	if strings.TrimSpace(input.Context) != "" {
		description = strings.TrimSpace(description + "\n\nRequested changes:\n" + input.Context)
	}
	return s.repo.Update(ctx, id, models.UpdateTaskInput{
		Description: &description,
		SpecStatus:  &specStatus,
		Status:      &status,
	})
}

func (s *TaskService) UpdateAnalysis(ctx context.Context, id string, analysis json.RawMessage) (*models.Task, error) {
	if !json.Valid(analysis) {
		return nil, ErrValidation("analysis must be valid JSON")
	}
	specStatus := models.TaskSpecStatusDraft
	return s.repo.Update(ctx, id, models.UpdateTaskInput{Analysis: analysis, SpecStatus: &specStatus})
}

func (s *TaskService) ListSubTasks(ctx context.Context, parentID string) ([]models.Task, error) {
	return s.repo.ListSubTasks(ctx, parentID)
}

func (s *TaskService) CreateSubTask(ctx context.Context, parentID string, input models.CreateTaskInput) (*models.Task, error) {
	parent, err := s.repo.GetByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	input.ParentTaskID = &parentID
	return s.Create(ctx, parent.ProjectID, input)
}

func buildTaskAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Derived from task title and description. Human review should refine this for Medium/Hard work.",
		AffectedFiles: []string{},
		Risks:         []string{"Analysis is heuristic until the Phase 3 planner agent is available."},
		ExecutionPlan: []string{
			"Confirm definition of ready.",
			"Identify affected files.",
			"Implement changes in an isolated worktree.",
			"Run automated tests before PR creation.",
		},
		ClarificationQuestions: questions,
		TaskRules:              []string{},
		ProposalMD:             fmt.Sprintf("## Proposal for %s\n\n%s\n", task.Title, task.Description),
		SpecsMD:                fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", task.Title, task.Description),
		DesignMD:               "## Design\n\nImplementation design placeholder.\n",
		TasksMD:                "## Tasks\n\n- [ ] Task execution workflow step\n",
	}
}
