package orchestrator

import (
	"context"

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
	switch task.Complexity {
	case models.TaskComplexityHard:
		return []string{models.AgentRolePlanner, models.AgentRoleBackend, models.AgentRoleReviewer}
	case models.TaskComplexityMedium:
		return []string{models.AgentRoleBackend, models.AgentRoleReviewer, models.AgentRolePlanner}
	default:
		return []string{models.AgentRolePlanner, models.AgentRoleBackend}
	}
}

func (m *AgentManager) Release(ctx context.Context, agentID string) error {
	return m.repo.UpdateStatus(ctx, agentID, models.AgentStatusIdle)
}
