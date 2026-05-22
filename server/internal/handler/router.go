package handler

import (
	"net/http"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps holds all service dependencies for the router.
type Deps struct {
	OrgSvc  *service.OrganizationService
	ProjSvc *service.ProjectService
	RepoSvc *service.RepositoryService
	AgentSvc *service.AgentService
	TaskSvc *service.TaskService
	RuleSvc *service.RuleService
	SkillSvc *service.SkillService
}

// NewRouter creates the chi router with all API v1 routes.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, envelope{"status": "ok", "version": "0.1.0"})
	})

	// Handlers
	orgH := NewOrganizationHandler(d.OrgSvc)
	projH := NewProjectHandler(d.ProjSvc)
	repoH := NewRepositoryHandler(d.RepoSvc)
	agentH := NewAgentHandler(d.AgentSvc)
	taskH := NewTaskHandler(d.TaskSvc)
	ruleH := NewRuleHandler(d.RuleSvc)
	skillH := NewSkillHandler(d.SkillSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Organizations
		r.Route("/organizations", func(r chi.Router) {
			r.Post("/", orgH.Create)
			r.Get("/", orgH.List)
			r.Route("/{orgID}", func(r chi.Router) {
				r.Get("/", orgH.GetByID)
				r.Patch("/", orgH.Update)
				r.Delete("/", orgH.Delete)

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
			r.Delete("/", projH.Delete)

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
		r.Get("/repositories/{repoID}", repoH.GetByID)
		r.Patch("/repositories/{repoID}", repoH.Update)
		r.Delete("/repositories/{repoID}", repoH.Delete)

		r.Get("/agents/{agentID}", agentH.GetByID)
		r.Patch("/agents/{agentID}", agentH.Update)
		r.Delete("/agents/{agentID}", agentH.Delete)

		r.Get("/tasks/{taskID}", taskH.GetByID)
		r.Patch("/tasks/{taskID}", taskH.Update)
		r.Delete("/tasks/{taskID}", taskH.Delete)

		r.Get("/rules/{ruleID}", ruleH.GetByID)
		r.Patch("/rules/{ruleID}", ruleH.Update)
		r.Delete("/rules/{ruleID}", ruleH.Delete)

		// Skills (global, not project-scoped)
		r.Route("/skills", func(r chi.Router) {
			r.Post("/", skillH.Create)
			r.Get("/", skillH.List)
			r.Get("/{skillID}", skillH.GetByID)
			r.Patch("/{skillID}", skillH.Update)
			r.Delete("/{skillID}", skillH.Delete)
		})
	})

	return r
}
