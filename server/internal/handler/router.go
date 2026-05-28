package handler

import (
	"net/http"
	"time"

	mw "github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps holds all service dependencies for the router.
type Deps struct {
	OrgSvc       *service.OrganizationService
	ProjSvc      *service.ProjectService
	RepoSvc      *service.RepositoryService
	AgentSvc     *service.AgentService
	TaskSvc      *service.TaskService
	RuleSvc      *service.RuleService
	SkillSvc     *service.SkillService
	AnalyticsSvc *service.AnalyticsService
	AuthSvc      *service.AuthService
	Orch         *orchestrator.Orchestrator
	WebPort      string
}

// NewRouter creates the chi router with all API v1 routes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	webPort := d.WebPort
	if webPort == "" {
		webPort = "32300"
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:" + webPort, "http://127.0.0.1:" + webPort},
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
		writeJSON(w, http.StatusOK, envelope{"status": "ok", "version": "0.2.0"})
	})

	// Handlers
	orgH := NewOrganizationHandler(d.OrgSvc)
	projH := NewProjectHandler(d.ProjSvc)
	repoH := NewRepositoryHandler(d.RepoSvc)
	agentH := NewAgentHandler(d.AgentSvc)
	taskH := NewTaskHandler(d.TaskSvc)
	ruleH := NewRuleHandler(d.RuleSvc)
	skillH := NewSkillHandler(d.SkillSvc)
	analyticsH := NewAnalyticsHandler(d.AnalyticsSvc)
	authH := NewAuthHandler(d.AuthSvc)
	webhookH := NewWebhookHandler(d.TaskSvc)
	workflowH := NewWorkflowHandler(d.Orch)

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
				})
			})

			// Standalone resource endpoints
			r.Get("/repositories/remote", repoH.ListRemoteRepos)
			r.Get("/repositories/{repoID}", repoH.GetByID)
			r.Patch("/repositories/{repoID}", repoH.Update)
			r.Delete("/repositories/{repoID}", repoH.Delete)
			r.Post("/repositories/{repoID}/validate", repoH.ValidateToken)
			r.Post("/repositories/{repoID}/clone", repoH.Clone)

			r.Get("/agents/{agentID}", agentH.GetByID)
			r.Patch("/agents/{agentID}", agentH.Update)
			r.Get("/agents/{agentID}/skills", skillH.ListAgentSkills)
			r.Post("/agents/{agentID}/skills", skillH.AssignToAgent)
			r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/agents/{agentID}", agentH.Delete)

			r.Get("/tasks/{taskID}", taskH.GetByID)
			r.Patch("/tasks/{taskID}", taskH.Update)
			r.Delete("/tasks/{taskID}", taskH.Delete)
			r.Post("/tasks/{taskID}/analyze", taskH.Analyze)
			r.Post("/tasks/{taskID}/clarify", taskH.Clarify)
			r.Get("/tasks/{taskID}/analysis", taskH.GetAnalysis)
			r.Patch("/tasks/{taskID}/analysis", taskH.UpdateAnalysis)
			r.Post("/tasks/{taskID}/analysis/approve", taskH.ApproveAnalysis)
			r.Post("/tasks/{taskID}/analysis/request-changes", taskH.RequestAnalysisChanges)
			r.Get("/tasks/{taskID}/subtasks", taskH.ListSubTasks)
			r.Post("/tasks/{taskID}/subtasks", taskH.CreateSubTask)
			r.Post("/tasks/{taskID}/execute", workflowH.Execute)
			r.Get("/tasks/{taskID}/logs", workflowH.Logs)
			r.Get("/tasks/{taskID}/workflow", workflowH.Status)
			r.Post("/tasks/{taskID}/approve", workflowH.Approve)

			r.Get("/rules/{ruleID}", ruleH.GetByID)
			r.Patch("/rules/{ruleID}", ruleH.Update)
			r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/rules/{ruleID}", ruleH.Delete)

			r.Get("/analytics/token-usage", analyticsH.TokenUsage)

			// Skills (global, not project-scoped)
			r.Route("/skills", func(r chi.Router) {
				r.Post("/", skillH.Create)
				r.Get("/", skillH.List)
				r.Get("/{skillID}", skillH.GetByID)
				r.Patch("/{skillID}", skillH.Update)
				r.Post("/{skillID}/test", skillH.Test)
				r.With(mw.RequireRole(models.UserRoleAdmin)).Delete("/{skillID}", skillH.Delete)
			})
		})
	})

	return r
}
