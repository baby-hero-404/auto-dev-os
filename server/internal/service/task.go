package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type TaskService struct {
	repo        *repository.TaskRepo
	projectRepo *repository.ProjectRepo
	repoRepo    *repository.RepositoryRepo
	orgRepo     *repository.OrganizationRepo
}

func NewTaskService(repo *repository.TaskRepo, projectRepo *repository.ProjectRepo, repoRepo *repository.RepositoryRepo, orgRepo *repository.OrganizationRepo) *TaskService {
	return &TaskService{repo: repo, projectRepo: projectRepo, repoRepo: repoRepo, orgRepo: orgRepo}
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
	if input.ExecutionEngine != nil && *input.ExecutionEngine != "" {
		if err := models.ValidateExecutionEngine(*input.ExecutionEngine); err != nil {
			return nil, ErrValidation(err.Error())
		}
		if err := s.validateTaskEngineOverride(ctx, projectID, *input.ExecutionEngine); err != nil {
			return nil, err
		}
	}
	task, err := s.repo.Create(ctx, projectID, input)
	if err != nil {
		return nil, err
	}
	return task, nil
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
	if input.ExecutionEngine != nil && *input.ExecutionEngine != "" {
		if err := models.ValidateExecutionEngine(*input.ExecutionEngine); err != nil {
			return nil, ErrValidation(err.Error())
		}
		task, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := s.validateTaskEngineOverride(ctx, task.ProjectID, *input.ExecutionEngine); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, id, input)
}

// validateTaskEngineOverride guards against setting a task's execution_engine
// override to "cli" when the owning project has no usable CLIEngineConfig
// (i.e. no command configured). Without this, a task-level override bypasses
// the equivalent check ProjectService already enforces when the project
// itself switches to engine "cli" — every run would then fail identically at
// the CLI engine's preflight step ("cli_engine_config.command is required").
func (s *TaskService) validateTaskEngineOverride(ctx context.Context, projectID, executionEngine string) error {
	if executionEngine != models.ExecutionEngineCLI {
		return nil
	}
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	// A project can satisfy a cli override either through the newer
	// ExecutionProviders list or the legacy CLIEngineConfig — check the
	// former first (mirroring legacyResolveExecutionProvider's "empty
	// providers -> use legacy field" fallback), otherwise a project that
	// only adopted ExecutionProviders gets rejected here even though the
	// Router would resolve it fine at Task run time.
	providers, err := models.ValidateExecutionProviders(project.ExecutionProviders)
	if err != nil {
		return ErrValidation(err.Error())
	}
	// See models.HasEnabledProvider's doc comment: "configured" means at
	// least one row enabled, not merely a non-empty list — must match
	// ResolveExecutionProvider's gate exactly, or this validation rejects
	// overrides the orchestrator would actually satisfy at Task run time.
	if models.HasEnabledProvider(providers) {
		for _, p := range providers {
			if p.Type == "cli" && p.Enabled {
				return nil
			}
		}
		return ErrValidation("task execution_engine cannot be set to \"cli\": project has no enabled cli execution provider")
	}
	// Same precedence as the orchestrator's ResolveExecutionProvider
	// (docs/openspecs/global-execution-providers): a project already
	// explicitly on the legacy cli path is validated against its own
	// CLIEngineConfig only, never the org default — so a project
	// deliberately configured this way behaves identically whether or not
	// an org default happens to exist.
	if project.ExecutionEngine != models.ExecutionEngineCLI && s.orgRepo != nil {
		if org, err := s.orgRepo.GetByID(ctx, project.OrgID); err == nil {
			if orgProviders, err := models.ValidateExecutionProviders(org.DefaultExecutionProviders); err == nil {
				for _, p := range orgProviders {
					if p.Type == "cli" && p.Enabled {
						return nil
					}
				}
			}
		}
	}
	var cfg models.CLIEngineConfig
	if len(project.CLIEngineConfig) > 0 {
		_ = json.Unmarshal(project.CLIEngineConfig, &cfg)
	}
	if err := models.ValidateCLIEngineConfig(executionEngine, &cfg); err != nil {
		return ErrValidation(fmt.Sprintf("task execution_engine cannot be set to \"cli\": project has no cli_engine_config configured (%s)", err.Error()))
	}
	return nil
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

	affectedFilesStrings := make([]string, len(analysis.AffectedFiles))
	for i, f := range analysis.AffectedFiles {
		affectedFilesStrings[i] = f.File
	}

	hasClarifications := len(analysis.ClarificationQuestions) > 0
	var priorRounds []models.ClarificationRound
	if len(task.Clarifications) > 0 {
		_ = json.Unmarshal(task.Clarifications, &priorRounds)
	}
	dorBypassed := hasClarifications && policy.IsDefinitionOfReadyBypassed(task.Labels, len(priorRounds))
	if dorBypassed {
		hasClarifications = false
	}

	specStatus, status := policy.ShouldAutoApproveSpec(
		analysis.Complexity,
		affectedFilesStrings,
		analysis.RiskDomains,
		"", // no agent autonomy in this path
		project.DefaultAutonomy,
		project.AutoReviewPolicy,
		hasClarifications,
	)
	if dorBypassed && specStatus == models.TaskSpecStatusAutoApproved {
		specStatus = models.TaskSpecStatusReadyWithWarnings
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

	var rounds []models.ClarificationRound
	if len(task.Clarifications) > 0 {
		_ = json.Unmarshal(task.Clarifications, &rounds)
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	newRound := models.ClarificationRound{
		Round:     len(rounds) + 1,
		Timestamp: time.Now(),
		Questions: analysis.ClarificationQuestions,
		Response:  input.Context,
	}
	rounds = append(rounds, newRound)
	clarificationsBytes, _ := json.Marshal(rounds)

	specStatus := models.TaskSpecStatusNone
	status := models.TaskStatusAnalyzing
	return s.repo.Update(ctx, id, models.UpdateTaskInput{
		SpecStatus:     &specStatus,
		Status:         &status,
		Clarifications: json.RawMessage(clarificationsBytes),
	})
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
		AffectedFiles: []models.AffectedFile{},
		Risks:         []string{"Analysis is heuristic until the Phase 3 planner agent is available."},
		ExecutionPhases: []models.ExecutionPhase{
			{
				Phase: "Setup and Execution",
				Tasks: []string{
					"Confirm definition of ready.",
					"Identify affected files.",
					"Implement changes in an isolated worktree.",
					"Run automated tests before PR creation.",
				},
			},
		},
		ClarificationQuestions: questions,
		TaskRules:              []string{},
		ProposalMD:             fmt.Sprintf("## Proposal for %s\n\n%s\n", task.Title, task.Description),
		SpecsMD:                fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", task.Title, task.Description),
		DesignMD:               "## Design\n\nImplementation design placeholder.\n",
		Tasks:                  []models.TaskDAG{{ID: "Task execution workflow step"}},
	}
}
