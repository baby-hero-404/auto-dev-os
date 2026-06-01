package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/database"
	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/handler"
	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/retrieval"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	shutdownTracing, err := observability.InitTracing(context.Background(), "auto-code-os-api", cfg.Telemetry.OTLPTraceEndpoint)
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			slog.Warn("shutdown tracing", "error", err)
		}
	}()

	// Run migrations
	migrationsPath, _ := filepath.Abs("migration")
	if err := database.Migrate(cfg.Database.URL, migrationsPath); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}

	// Wire: repos → services → handlers
	orgRepo := repository.NewOrganizationRepo(db)
	projRepo := repository.NewProjectRepo(db)
	repoRepo := repository.NewRepositoryRepo(db)
	agentRepo := repository.NewAgentRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	ruleRepo := repository.NewRuleRepo(db)
	skillRepo := repository.NewSkillRepo(db)
	authRepo := repository.NewAuthRepo(db)
	workflowRepo := repository.NewWorkflowRepo(db)
	secretRepo := repository.NewSecretRepo(db)
	analyticsRepo := repository.NewAnalyticsRepo(db)
	dashboardRepo := repository.NewAnalyticsDashboardRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	memoryRepo := repository.NewMemoryRepo(db)
	edgeRepo := repository.NewKnowledgeEdgeRepo(db)
	suggestionRepo := repository.NewLearningSuggestionRepo(db)

	if _, err := service.NewSecretService(secretRepo, cfg.Auth.JWTSecret); err != nil {
		return err
	}
	sandboxRuntime, err := buildSandboxRuntime(cfg)
	if err != nil {
		return err
	}
	agentManager := orchestrator.NewAgentManager(agentRepo)
	promptAssembler := orchestrator.NewPromptAssemblerWithRules(
		retrieval.NewSimpleFileRetriever("."),
		ruleRepo,
		filepath.Clean(filepath.Join("..", "resources", "prompt_base")),
	)
	orch := orchestrator.NewOrchestratorWithPrompt(taskRepo, workflowRepo, agentManager, sandboxRuntime, promptAssembler)
	artifactRepo := repository.NewArtifactRepo(db)
	orch.SetArtifactRepository(artifactRepo)
	gitProvider := gitops.NewGitHubProvider()
	gitOpsAdapter := gitops.NewGitOpsAdapter(gitProvider, repoRepo, cfg.Sandbox.WorkspaceRoot)
	orch.SetGitOpsClient(gitOpsAdapter)
	orch.SetRepositoryRepository(repoRepo)
	orch.SetWorkspaceRoot(cfg.Sandbox.WorkspaceRoot)
	if provider, err := buildLLMProvider(cfg); err != nil {
		slog.Warn("llm provider disabled", "error", err)
	} else if provider != nil {
		orch.SetLLMProvider(provider)
	}

	// Phase 6: Memory & Learning
	memorySvc := service.NewMemoryService(memoryRepo, edgeRepo)
	learningSvc := service.NewLearningService(suggestionRepo, ruleRepo)
	memoryHooks := orchestrator.NewMemoryHooks(memorySvc)
	learningEngine := orchestrator.NewLearningEngine(memorySvc, learningSvc, taskRepo)
	// Attach hooks to orchestrator
	orch.SetMemoryHooks(memoryHooks)
	orch.SetLearningEngine(learningEngine)

	deps := handler.Deps{
		OrgSvc:             service.NewOrganizationService(orgRepo),
		ProjSvc:            service.NewProjectService(projRepo, service.NewSeederService(ruleRepo, skillRepo)),
		RepoSvc:            service.NewRepositoryService(repoRepo),
		AgentSvc:           service.NewAgentService(agentRepo),
		TaskSvc:            service.NewTaskService(taskRepo),
		RuleSvc:            service.NewRuleService(ruleRepo),
		SkillSvc:           service.NewSkillService(skillRepo),
		AnalyticsSvc:       service.NewAnalyticsService(analyticsRepo),
		DashboardSvc:       service.NewAnalyticsDashboardService(dashboardRepo),
		AuditSvc:           service.NewAuditService(auditRepo),
		AuthSvc:            service.NewAuthService(authRepo, cfg.Auth.JWTSecret),
		MemorySvc:          memorySvc,
		LearningSvc:        learningSvc,
		Orch:               orch,
		WebPort:            cfg.Server.WebPort,
		CORSAllowedOrigins: cfg.Server.CORSOrigins,
	}

	router := handler.NewRouter(deps)

	// HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	if cfg.Worker.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orch.StartWorker(workerCtx, time.Duration(cfg.Worker.IntervalMS)*time.Millisecond, cfg.Worker.Concurrency)
		}()
		slog.Info("workflow queue worker started", "interval_ms", cfg.Worker.IntervalMS, "concurrency", cfg.Worker.Concurrency)
	}
	go func() {
		slog.Info("api server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stopWorker()
	wg.Wait()
	orch.Wait()
	return srv.Shutdown(ctx)
}

func buildLLMProvider(cfg *config.Config) (llm.Provider, error) {
	switch cfg.LLM.Provider {
	case "gateway":
		return llm.NewProvider(cfg)
	case "9router":
		if cfg.LLM.APIKey == "" && cfg.LLM.LLMAPIKey == "" {
			return nil, nil
		}
		return llm.NewProvider(cfg)
	default:
		if cfg.LLM.APIKey == "" {
			return nil, nil
		}
		return llm.NewProvider(cfg)
	}
}

func buildSandboxRuntime(cfg *config.Config) (sandbox.Runtime, error) {
	switch cfg.Sandbox.Runtime {
	case "docker":
		runtime, err := sandbox.NewDockerRuntime(sandbox.DockerConfig{
			Image:             cfg.Sandbox.Image,
			WorkspaceRoot:     cfg.Sandbox.WorkspaceRoot,
			MemoryBytes:       cfg.Sandbox.MemoryMB * 1024 * 1024,
			NanoCPUs:          cfg.Sandbox.NanoCPUs,
			DisableNetworking: true,
		})
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runtime.Health(ctx); err != nil {
			return nil, err
		}
		return runtime, nil
	case "", "stub":
		return sandbox.NewStubRuntime(), nil
	default:
		return nil, fmt.Errorf("unsupported SANDBOX_RUNTIME %q", cfg.Sandbox.Runtime)
	}
}
