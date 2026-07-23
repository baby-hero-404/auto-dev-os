package handler

import (
	"net/http"
	"strings"
	"time"

	mw "github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Version is the build version of the application.
// Default "dev" is overridden by config.yaml at startup, or by -ldflags at build time.
var Version = "dev"

// Deps holds all service dependencies for the router.
type Deps struct {
	OrgSvc          OrganizationService
	ProjSvc         ProjectService
	RepoSvc         RepositoryService
	AgentSvc        AgentService
	TaskSvc         TaskService
	RuleSvc         RuleService
	SkillSvc        SkillService
	LearnedSkillSvc LearnedSkillService
	AnalyticsSvc    AnalyticsService
	DashboardSvc    AnalyticsDashboardService
	AuditSvc        AuditService
	AuthSvc         AuthService
	MemorySvc       MemoryService
	LearningSvc     LearningService
	GitAccountSvc   GitAccountService
	ProviderCredSvc ProviderCredentialService

	ProviderModelSvc   ProviderModelService
	AttestationSvc     AttestationService
	Orch               *orchestrator.Orchestrator
	WebPort            string
	CORSAllowedOrigins string
}

// NewRouter creates the chi router with all API v1 routes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(observability.TraceMiddleware("auto-code-os-api"))
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	webPort := d.WebPort
	if webPort == "" {
		webPort = "32300"
	}
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:" + webPort, "http://127.0.0.1:" + webPort}
	if strings.TrimSpace(d.CORSAllowedOrigins) != "" {
		allowedOrigins = splitCSV(d.CORSAllowedOrigins)
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Rate limiter: 60 requests per second, burst of 120.
	limiter := mw.NewRateLimiter(60, 120, time.Second)
	r.Use(mw.InjectClaimsFromJWT)
	r.Use(mw.RateLimit(limiter))

	// Health check
	r.Get("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, envelope{"status": "ok", "version": Version})
	})

	// Handlers
	orgH := NewOrganizationHandler(d.OrgSvc)
	projH := NewProjectHandler(d.ProjSvc)
	repoH := NewRepositoryHandler(d.RepoSvc)
	agentH := NewAgentHandler(d.AgentSvc)
	taskH := NewTaskHandler(d.TaskSvc, d.Orch)
	ruleH := NewRuleHandler(d.RuleSvc)
	skillH := NewSkillHandler(d.SkillSvc)
	learnedSkillH := NewLearnedSkillHandler(d.LearnedSkillSvc)
	analyticsH := NewAnalyticsHandler(d.AnalyticsSvc)
	dashboardH := NewAnalyticsDashboardHandler(d.DashboardSvc)
	attestationH := NewAttestationHandler(d.AttestationSvc)
	auditH := NewAuditHandler(d.AuditSvc)
	prH := NewPRHandler(d.TaskSvc, d.AuditSvc, d.Orch)
	authH := NewAuthHandler(d.AuthSvc)
	webhookH := NewWebhookHandler(d.TaskSvc, d.Orch)
	workflowH := NewWorkflowHandler(d.Orch)
	memoryH := NewMemoryHandler(d.MemorySvc)
	learningH := NewLearningHandler(d.LearningSvc)
	gitAccH := NewGitAccountHandler(d.GitAccountSvc)
	providerCredH := NewProviderCredentialHandler(d.ProviderCredSvc)

	providerModelH := NewProviderModelHandler(d.ProviderModelSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Public: auth endpoints
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)
		})
		// Public: webhook (token-based auth in handler)
		r.Post("/webhooks/github", webhookH.GitHub)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(d.AuthSvc))

			// Organizations — admin-only for create/update/delete
			r.Route("/organizations", func(r chi.Router) {
				r.Get("/", orgH.List)
				r.With(mw.RequireRole(models.UserRoleAdmin)).Post("/", orgH.Create)
				r.Route("/{orgID}", func(r chi.Router) {
					r.Get("/", orgH.GetByID)
					r.With(mw.RequireRole(models.UserRoleAdmin)).Patch("/", orgH.Update)
					r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/", orgH.Delete)

					r.Route("/agents", func(r chi.Router) {
						r.With(mw.RequireRole(models.UserRoleAdmin)).Post("/", agentH.Hire)
						r.Get("/", agentH.ListOrg)
					})

					// Nested: projects under org
					r.Route("/projects", func(r chi.Router) {
						r.Post("/", projH.Create)
						r.Get("/", projH.List)
					})

					r.Route("/rules", func(r chi.Router) {
						r.Get("/", ruleH.ListGlobal)
						r.With(mw.RequireRole(models.UserRoleAdmin)).Post("/", ruleH.CreateGlobal)
						r.With(mw.RequireRole(models.UserRoleAdmin)).Post("/seed", ruleH.SeedGlobal)
					})

					r.Route("/git-accounts", func(r chi.Router) {
						r.Post("/", gitAccH.Create)
						r.Get("/", gitAccH.List)
					})

					r.Route("/provider-credentials", func(r chi.Router) {
						r.Get("/", providerCredH.List)
						r.Group(func(r chi.Router) {
							r.Use(mw.RequireRole(models.UserRoleAdmin))
							r.Post("/", providerCredH.Create)
							r.Post("/test", providerCredH.TestInput)
							r.Put("/{credentialID}", providerCredH.Update)
							r.Delete("/{credentialID}", providerCredH.Delete)
							r.Post("/{credentialID}/test", providerCredH.Test)
						})
					})

					r.Route("/provider-models", func(r chi.Router) {
						r.Get("/", providerModelH.List)
						r.Group(func(r chi.Router) {
							r.Use(mw.RequireRole(models.UserRoleAdmin))
							r.Post("/", providerModelH.Create)
							r.Put("/{modelID}", providerModelH.Update)
							r.Delete("/{modelID}", providerModelH.Delete)
						})
					})
				})
			})

			// Projects (standalone)
			r.Route("/projects/{projectID}", func(r chi.Router) {
				r.Get("/", projH.GetByID)
				r.Patch("/", projH.Update)
				r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/", projH.Delete)

				// Nested: repos, agents, tasks, rules
				r.Route("/repositories", func(r chi.Router) {
					r.Post("/", repoH.Create)
					r.Get("/", repoH.List)
				})
				r.Route("/agents", func(r chi.Router) {
					r.Post("/", agentH.Create)
					r.Get("/", agentH.List)
				})
				r.Route("/tasks", func(r chi.Router) {
					r.Post("/", taskH.Create)
					r.Get("/", taskH.List)
				})
				r.Route("/rules", func(r chi.Router) {
					r.Post("/", ruleH.Create)
					r.Get("/", ruleH.List)
					r.Post("/seed", ruleH.Seed)
				})
			})

			// Standalone resource endpoints
			r.Get("/repositories/remote", repoH.ListRemoteRepos)
			r.Post("/repositories/branches", repoH.GetBranches)
			r.Get("/repositories/{repoID}", repoH.GetByID)
			r.Patch("/repositories/{repoID}", repoH.Update)
			r.Delete("/repositories/{repoID}", repoH.Delete)
			r.Post("/repositories/{repoID}/validate", repoH.ValidateToken)
			r.Post("/repositories/{repoID}/clone", repoH.Clone)

			r.Get("/git-accounts/{accID}", gitAccH.GetByID)
			r.Patch("/git-accounts/{accID}", gitAccH.Update)
			r.Delete("/git-accounts/{accID}", gitAccH.Delete)
			r.Post("/git-accounts/{accID}/test", gitAccH.Test)

			r.Get("/agents/{agentID}", agentH.GetByID)
			r.Patch("/agents/{agentID}", agentH.Update)
			r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/agents/{agentID}", agentH.Delete)
			r.Get("/role-templates", agentH.ListRoleTemplates)

			// Phase 6: Episodic Memory
			r.Get("/agents/{agentID}/memories", memoryH.ListByAgent)
			r.Get("/agents/{agentID}/memories/search", memoryH.Search)

			// Phase 6: Learning Suggestions
			r.Get("/agents/{agentID}/suggestions", learningH.ListSuggestions)

			r.Get("/tasks/{taskID}", taskH.GetByID)
			r.Patch("/tasks/{taskID}", taskH.Update)
			r.Delete("/tasks/{taskID}", taskH.Delete)
			r.Post("/tasks/{taskID}/analyze", taskH.Analyze)
			r.Post("/tasks/{taskID}/clarify", taskH.Clarify)
			r.Get("/tasks/{taskID}/analysis", taskH.GetAnalysis)
			r.Patch("/tasks/{taskID}/analysis", taskH.UpdateAnalysis)
			r.Post("/tasks/{taskID}/analysis/approve", taskH.ApproveAnalysis)
			r.Post("/tasks/{taskID}/analysis/request-changes", taskH.RequestAnalysisChanges)
			r.Post("/tasks/{taskID}/spec-review", taskH.SpecReview)
			r.Get("/tasks/{taskID}/spec", taskH.GetSpec)
			r.Get("/tasks/{taskID}/subtasks", taskH.ListSubTasks)
			r.Post("/tasks/{taskID}/subtasks", taskH.CreateSubTask)
			r.Post("/tasks/{taskID}/execute", workflowH.Execute)
			r.Get("/tasks/{taskID}/logs", workflowH.Logs)
			r.Get("/tasks/{taskID}/logs/stream", workflowH.StreamLogs)
			r.Get("/tasks/{taskID}/workflow", workflowH.Status)
			r.Post("/tasks/{taskID}/approve", workflowH.Approve)
			r.Post("/tasks/{taskID}/restart", workflowH.Retry)
			r.Post("/tasks/{taskID}/retry", workflowH.Retry)
			r.Post("/tasks/{taskID}/pause", workflowH.Pause)
			r.Post("/tasks/{taskID}/cancel", workflowH.Cancel)
			r.Post("/tasks/{taskID}/close", workflowH.Cancel)
			r.Get("/workflows/{jobID}/artifacts", workflowH.Artifacts)
			r.Get("/tasks/{taskID}/attestations", attestationH.ListByTask)

			r.Get("/attestations/keys", attestationH.Keys)
			r.Get("/attestations/{commit}", attestationH.GetByCommit)

			r.Get("/rules/{ruleID}", ruleH.GetByID)
			r.Patch("/rules/{ruleID}", ruleH.Update)
			r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/rules/{ruleID}", ruleH.Delete)

			r.Get("/analytics/token-usage", analyticsH.TokenUsage)
			r.Get("/analytics/overview", dashboardH.Overview)
			r.Get("/analytics/agents", dashboardH.AgentPerformance)
			r.Get("/analytics/tasks", dashboardH.TaskAnalytics)
			r.Get("/analytics/gateway-usage", dashboardH.GatewayUsage)
			r.Get("/analytics/workflows", dashboardH.WorkflowAnalytics)
			r.Get("/analytics/failures", dashboardH.RecentFailures)

			// Audit logs
			r.Get("/audit/logs", auditH.List)
			r.Get("/audit/summary", auditH.Summary)

			// PR approval/rejection
			r.Post("/tasks/{taskID}/pr/approve", prH.Approve)
			r.Post("/tasks/{taskID}/pr/reject", prH.Reject)
			r.Post("/tasks/{taskID}/pr/start-review", prH.StartReview)

			// Phase 6: Memory detail
			r.Get("/memories/{memoryID}", memoryH.GetByID)
			r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/memories/{memoryID}", memoryH.Delete)

			// Phase 6: Learning suggestion review
			r.Get("/suggestions/{suggestionID}", learningH.GetSuggestion)
			r.Post("/suggestions/{suggestionID}/approve", learningH.ApproveSuggestion)
			r.Post("/suggestions/{suggestionID}/reject", learningH.RejectSuggestion)

			// Skills (global, not project-scoped)
			r.Route("/skills", func(r chi.Router) {
				r.Get("/", skillH.List)
				r.Post("/seed", skillH.Seed)
				r.Get("/sources", skillH.ListSources)
				r.Post("/sources", skillH.AddSource)
				r.Delete("/sources/{sourceID}", skillH.DeleteSource)
				r.Post("/sources/{sourceID}/sync", skillH.SyncSource)
				r.Get("/sources/{sourceID}/files", skillH.ListFiles)
				r.Get("/sources/{sourceID}/file-content", skillH.GetFileContent)
				r.Get("/{skillID}", skillH.GetByID)
				r.Post("/{skillID}/test", skillH.Test)
			})

			// Learned skills (reusable-skills-system): project-scoped, distinct
			// from the tool/plugin catalog above.
			r.Get("/projects/{projectID}/learned-skills", learnedSkillH.ListLearnedSkills)
			r.Route("/learned-skills", func(r chi.Router) {
				r.Get("/{skillID}", learnedSkillH.GetLearnedSkill)
				r.Patch("/{skillID}", learnedSkillH.UpdateLearnedSkill)
				r.Delete("/{skillID}", learnedSkillH.DeleteLearnedSkill)
			})
		})
	})

	return r
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}
