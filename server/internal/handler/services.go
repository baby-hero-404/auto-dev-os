package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type OrganizationService interface {
	Create(context.Context, models.CreateOrganizationInput) (*models.Organization, error)
	GetByID(context.Context, string) (*models.Organization, error)
	List(context.Context) ([]models.Organization, error)
	Update(context.Context, string, models.UpdateOrganizationInput) (*models.Organization, error)
	Delete(context.Context, string) error
}

type ProjectService interface {
	Create(context.Context, string, models.CreateProjectInput) (*models.Project, error)
	GetByID(context.Context, string) (*models.Project, error)
	ListByOrgID(context.Context, string) ([]models.Project, error)
	Update(context.Context, string, models.UpdateProjectInput) (*models.Project, error)
	Delete(context.Context, string) error
}

type RepositoryService interface {
	ValidateToken(context.Context, string) error
	ListRemoteRepos(context.Context, string) ([]models.RemoteRepository, error)
	Clone(context.Context, string) (*models.Repository, error)
	Create(context.Context, string, models.CreateRepositoryInput) (*models.Repository, error)
	GetByID(context.Context, string) (*models.Repository, error)
	ListByProjectID(context.Context, string) ([]models.Repository, error)
	Update(context.Context, string, models.UpdateRepositoryInput) (*models.Repository, error)
	Delete(context.Context, string) error
	GetRemoteBranches(context.Context, string, string, *string) ([]string, error)
}

type AgentService interface {
	AssignToProject(context.Context, string, models.CreateAgentInput) (*models.Agent, error)
	Hire(context.Context, string, models.CreateAgentInput) (*models.Agent, error)
	GetByID(context.Context, string) (*models.Agent, error)
	ListByProjectID(context.Context, string) ([]models.Agent, error)
	ListByOrgID(context.Context, string) ([]models.Agent, error)
	ListRoleTemplates(context.Context) ([]models.RoleTemplate, error)
	Update(context.Context, string, models.UpdateAgentInput) (*models.Agent, error)
	Delete(context.Context, string) error
}

type TaskService interface {
	Create(context.Context, string, models.CreateTaskInput) (*models.Task, error)
	GetByID(context.Context, string) (*models.Task, error)
	ListByProjectID(context.Context, string) ([]models.Task, error)
	Update(context.Context, string, models.UpdateTaskInput) (*models.Task, error)
	Delete(context.Context, string) error
	Analyze(context.Context, string) (*models.Task, error)
	Clarify(context.Context, string, models.ClarifyTaskInput) (*models.Task, error)
	GetAnalysis(context.Context, string) (json.RawMessage, error)
	ApproveAnalysis(context.Context, string) (*models.Task, error)
	RequestAnalysisChanges(context.Context, string, models.ClarifyTaskInput) (*models.Task, error)
	UpdateAnalysis(context.Context, string, json.RawMessage) (*models.Task, error)
	ListSubTasks(context.Context, string) ([]models.Task, error)
	CreateSubTask(context.Context, string, models.CreateTaskInput) (*models.Task, error)
}

type RuleService interface {
	Create(context.Context, *string, models.CreateRuleInput) (*models.Rule, error)
	CreateGlobal(context.Context, string, models.CreateRuleInput) (*models.Rule, error)
	GetByID(context.Context, string) (*models.Rule, error)
	ListByProjectID(context.Context, string) ([]models.Rule, error)
	ListGlobalByOrgID(context.Context, string) ([]models.Rule, error)
	Update(context.Context, string, models.UpdateRuleInput) (*models.Rule, error)
	Delete(context.Context, string) error
	SeedDefaultRules(context.Context, string) ([]models.Rule, error)
	SeedGlobalDefaultRules(context.Context, string) ([]models.Rule, error)
}

type SkillService interface {
	Create(context.Context, models.CreateSkillInput) (*models.Skill, error)
	GetByID(context.Context, string) (*models.Skill, error)
	List(context.Context) ([]models.Skill, error)
	Test(context.Context, string, map[string]any) (map[string]any, error)
	Update(context.Context, string, models.UpdateSkillInput) (*models.Skill, error)
	Delete(context.Context, string) error
	SeedDefaultSkills(context.Context) ([]models.Skill, error)
}

type AnalyticsService interface {
	TokenUsage(context.Context, string, time.Time) ([]models.TokenUsageSummary, error)
}

type AnalyticsDashboardService interface {
	Overview(context.Context, string) (*models.OverviewStats, error)
	AgentPerformance(context.Context, string) ([]models.AgentStats, error)
	TaskAnalytics(context.Context, string, int) (*models.TaskAnalytics, error)
	WorkflowAnalytics(context.Context, string) (*models.WorkflowAnalytics, error)
}

type AuditService interface {
	RecordAction(context.Context, string, string, string, ...service.AuditOption)
	List(context.Context, models.AuditLogFilter) ([]models.AuditLog, error)
	CountByAction(context.Context, string) (map[string]int64, error)
}

type AuthService interface {
	Register(context.Context, models.RegisterInput) (*models.AuthResponse, error)
	Login(context.Context, models.LoginInput) (*models.AuthResponse, error)
	Refresh(context.Context, string) (*models.AuthResponse, error)
	VerifyToken(string, string) (*service.TokenClaims, error)
}

type MemoryService interface {
	ListByAgent(context.Context, string, string, int, int) ([]models.EpisodicMemory, error)
	Search(context.Context, models.MemorySearchInput) ([]models.MemorySearchResult, error)
	GetByID(context.Context, string) (*models.EpisodicMemory, error)
	Delete(context.Context, string) error
	GetEdgesByMemory(context.Context, string) ([]models.KnowledgeEdge, error)
}

type LearningService interface {
	ListSuggestions(context.Context, string, string, int) ([]models.LearningSuggestion, error)
	GetSuggestion(context.Context, string) (*models.LearningSuggestion, error)
	ApproveSuggestion(context.Context, string, string) (*models.LearningSuggestion, error)
	RejectSuggestion(context.Context, string, string, string) (*models.LearningSuggestion, error)
}

type GitAccountService interface {
	Create(context.Context, string, models.CreateGitAccountInput) (*models.GitAccount, error)
	GetByID(context.Context, string) (*models.GitAccount, error)
	ListByOrgID(context.Context, string) ([]models.GitAccount, error)
	Update(context.Context, string, models.UpdateGitAccountInput) (*models.GitAccount, error)
	Delete(context.Context, string) error
	TestConnection(context.Context, string) error
}

type ProviderCredentialService interface {
	Create(context.Context, string, models.CreateProviderCredentialInput) (*models.ProviderCredentialResponse, error)
	ListByOrg(context.Context, string) ([]models.ProviderCredentialResponse, error)
	Update(context.Context, string, models.UpdateProviderCredentialInput) (*models.ProviderCredentialResponse, error)
	Delete(context.Context, string) error
	TestConnection(context.Context, string) error
	TestConnectionInput(context.Context, models.TestProviderCredentialInput) error
}



type ModelRouteService interface {
	Create(context.Context, string, models.CreateModelRouteInput) (*models.ModelRoute, error)
	ListByOrg(context.Context, string) ([]models.ModelRoute, error)
	Update(context.Context, string, models.UpdateModelRouteInput) (*models.ModelRoute, error)
	Delete(context.Context, string) error
}
