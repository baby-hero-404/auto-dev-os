package orchestrator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type AgentManager struct {
	repo *repository.AgentRepo
}

func NewAgentManager(repo *repository.AgentRepo) *AgentManager {
	return &AgentManager{repo: repo}
}

func (m *AgentManager) Assign(ctx context.Context, task *models.Task) (*models.Agent, error) {
	var agent *models.Agent
	var err error
	for _, role := range rolesForTask(task) {
		agent, err = m.repo.FindAvailableByRole(ctx, task.ProjectID, role)
		if err == nil {
			break
		}
	}
	if agent == nil {
		agent, err = m.repo.FindAnyAvailable(ctx, task.ProjectID)
		if err != nil {
			return nil, err
		}
	}
	status := models.AgentStatusAssigned
	if err := m.repo.UpdateStatus(ctx, agent.ID, status); err != nil {
		return nil, err
	}
	agent.Status = status
	return agent, nil
}

func (m *AgentManager) AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error) {
	agent, err := m.repo.FindAvailableByRole(ctx, task.ProjectID, models.AgentRoleReviewer)
	if err != nil {
		agent, err = m.repo.FindAnyAvailable(ctx, task.ProjectID)
		if err != nil {
			return nil, err
		}
	}
	status := models.AgentStatusAssigned
	if err := m.repo.UpdateStatus(ctx, agent.ID, status); err != nil {
		return nil, err
	}
	agent.Status = status
	return agent, nil
}

func (m *AgentManager) AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	agent, err := m.repo.FindAvailableByRole(ctx, task.ProjectID, models.AgentRoleBackend)
	if err != nil {
		agent, err = m.repo.FindAnyAvailable(ctx, task.ProjectID)
		if err != nil {
			return nil, err
		}
	}
	status := models.AgentStatusAssigned
	if err := m.repo.UpdateStatus(ctx, agent.ID, status); err != nil {
		return nil, err
	}
	agent.Status = status
	return agent, nil
}

func (m *AgentManager) AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	agent, err := m.repo.FindAvailableByRole(ctx, task.ProjectID, models.AgentRoleFrontend)
	if err != nil {
		agent, err = m.repo.FindAnyAvailable(ctx, task.ProjectID)
		if err != nil {
			return nil, err
		}
	}
	status := models.AgentStatusAssigned
	if err := m.repo.UpdateStatus(ctx, agent.ID, status); err != nil {
		return nil, err
	}
	agent.Status = status
	return agent, nil
}

func (m *AgentManager) MarkRunning(ctx context.Context, agentID string) error {
	return m.repo.UpdateStatus(ctx, agentID, models.AgentStatusRunning)
}

func rolesForTask(task *models.Task) []string {
	// 1. Initial phases: Always use Planner
	if task.Status == "" || task.Status == models.TaskStatusTodo || task.Status == models.TaskStatusContextLoading || task.Status == models.TaskStatusAnalyzing || task.Status == models.TaskStatusSpecReview {
		return []string{models.AgentRolePlanner, models.AgentRoleBackend, models.AgentRoleFrontend}
	}

	// 2. Reviewing phases: Always use Reviewer
	if task.Status == models.TaskStatusReviewing || task.Status == models.TaskStatusHumanReview {
		return []string{models.AgentRoleReviewer, models.AgentRolePlanner, models.AgentRoleBackend}
	}

	// 3. Execution phases: Determine role by PrimaryCategory from Analysis
	primaryRole := models.AgentRoleBackend // fallback
	if len(task.Analysis) > 0 {
		var analysis models.TaskAnalysis
		if err := json.Unmarshal(task.Analysis, &analysis); err == nil {
			switch strings.ToLower(analysis.PrimaryCategory) {
			case "frontend", "ui", "ux":
				primaryRole = models.AgentRoleFrontend
			case "database", "db":
				primaryRole = models.AgentRoleDBArchitect
			case "qa", "testing":
				primaryRole = models.AgentRoleQA
			case "security":
				primaryRole = models.AgentRoleSecurityAuditor
			}
		}
	}

	roles := []string{primaryRole, models.AgentRolePlanner}
	if primaryRole != models.AgentRoleBackend {
		roles = append(roles, models.AgentRoleBackend)
	}
	return roles
}

func (m *AgentManager) Release(ctx context.Context, agentID string) error {
	return m.repo.UpdateStatus(ctx, agentID, models.AgentStatusIdle)
}
