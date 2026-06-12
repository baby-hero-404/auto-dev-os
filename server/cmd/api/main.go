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
	aigateway "github.com/auto-code-os/auto-code-os/server/internal/gateway"
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
	db, err := database.ConnectWithPool(cfg.Database.URL, database.PoolConfig{
		MaxOpenConns:           cfg.Database.MaxOpenConns,
		MaxIdleConns:           cfg.Database.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.Database.ConnMaxLifetimeSeconds,
		ConnMaxIdleTimeSeconds: cfg.Database.ConnMaxIdleTimeSeconds,
	})
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}

	// Wire: repos → services → handlers
	orgRepo := repository.NewOrganizationRepo(db)
	projRepo := repository.NewProjectRepo(db)
	repoRepo := repository.NewRepositoryRepo(db)
	agentRepo := repository.NewAgentRepo(db)
	roleTemplateRepo := repository.NewRoleTemplateRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	ruleRepo := repository.NewRuleRepo(db)
	skillRepo := repository.NewSkillRepo(db)
	authRepo := repository.NewAuthRepo(db)
	workflowRepo := repository.NewWorkflowRepo(db)
	workflowRepo.SetLogFileRoot(cfg.Logging.FileRoot)
	secretRepo := repository.NewSecretRepo(db)
	analyticsRepo := repository.NewAnalyticsRepo(db)
	dashboardRepo := repository.NewAnalyticsDashboardRepo(db)
	auditRepo := repository.NewAuditRepo(db)
	memoryRepo := repository.NewMemoryRepo(db)
	edgeRepo := repository.NewKnowledgeEdgeRepo(db)
	suggestionRepo := repository.NewLearningSuggestionRepo(db)
	gitAccountRepo := repository.NewGitAccountRepo(db)
	providerCredentialRepo := repository.NewProviderCredentialRepo(db)
	virtualKeyRepo := repository.NewVirtualKeyRepo(db)
	modelRouteRepo := repository.NewModelRouteRepo(db)

	secretCipher, err := service.NewSecretCipher(cfg.Auth.JWTSecret)
	if err != nil {
		return err
	}
	if _, err := service.NewSecretService(secretRepo, cfg.Auth.JWTSecret); err != nil {
		return err
	}
	auditSvc := service.NewAuditService(auditRepo)
	credentialPoolSvc := service.NewCredentialPoolService(providerCredentialRepo, secretCipher).WithAudit(auditSvc)
	virtualKeySvc := service.NewVirtualKeyService(virtualKeyRepo).WithAudit(auditSvc)
	modelRouteSvc := service.NewModelRouteService(modelRouteRepo)
	sandboxRuntime, err := buildSandboxRuntime(cfg)
	if err != nil {
		return err
	}

	// Sandbox Registry Pre-warming
	// Pull standard runtime images asynchronously on service start to reduce first Task Agent run latency.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		slog.Info("sandbox pre-warming started", "runtime", cfg.Sandbox.Runtime)
		if err := sandboxRuntime.Prewarm(ctx); err != nil {
			slog.Warn("sandbox pre-warming failed", "error", err)
		} else {
			slog.Info("sandbox pre-warming completed successfully")
		}
	}()
	agentManager := orchestrator.NewAgentManager(agentRepo)
	promptAssembler := orchestrator.NewPromptAssemblerWithRules(
		retrieval.NewSimpleFileRetriever("."),
		ruleRepo,
		filepath.Clean(filepath.Join("..", "resources", "prompt_base")),
	).WithSkillLister(skillRepo)
	orch := orchestrator.NewOrchestratorWithPrompt(taskRepo, workflowRepo, agentManager, sandboxRuntime, promptAssembler)
	artifactRepo := repository.NewArtifactRepo(db)
	orch.SetArtifactRepository(artifactRepo)
	gitProvider := gitops.NewGitHubProvider("")
	gitOpsAdapter := gitops.NewGitOpsAdapter(gitProvider, repoRepo, cfg.Sandbox.WorkspaceRoot)
	gitOpsAdapter.SetGitAccountLookup(gitAccountRepo)
	orch.SetGitOpsClient(gitOpsAdapter)
	orch.SetRepositoryRepository(repoRepo)
	orch.SetWorkspaceRoot(cfg.Sandbox.WorkspaceRoot)
	orch.SetWorkspaceRetention(
		time.Duration(cfg.Sandbox.WorkspaceRetentionHours)*time.Hour,
		time.Duration(cfg.Sandbox.WorkspaceCleanupIntervalMinutes)*time.Minute,
	)
	if provider, err := buildLLMProvider(cfg, credentialPoolSvc, virtualKeySvc, modelRouteSvc); err != nil {
		slog.Warn("llm provider disabled", "error", err)
	} else if provider != nil {
		orch.SetLLMProvider(provider)
	}

	// Phase 6: Memory & Learning
	memorySvc := service.NewMemoryService(memoryRepo, edgeRepo)
	if cfg.LLM.OpenAIAPIKey != "" {
		memorySvc.SetEmbedder(llm.NewOpenAIEmbedder(cfg.LLM.OpenAIAPIKey, cfg.LLM.EmbeddingModel))
	} else {
		slog.Warn("memory embeddings disabled: OPENAI_API_KEY is not configured")
	}
	learningSvc := service.NewLearningService(suggestionRepo, ruleRepo)
	learningSvc.SetSkillRepo(skillRepo)
	learningSvc.SetPromptRoot(filepath.Clean(filepath.Join("..", "resources", "prompt_base")))
	memoryHooks := orchestrator.NewMemoryHooks(memorySvc)
	learningEngine := orchestrator.NewLearningEngine(memorySvc, learningSvc, taskRepo)
	// Attach hooks to orchestrator
	orch.SetMemoryHooks(memoryHooks)
	orch.SetLearningEngine(learningEngine)

	repoSvc := service.NewRepositoryService(repoRepo)
	repoSvc.SetProjectRepo(projRepo)
	repoSvc.SetGitAccountRepo(gitAccountRepo)

	deps := handler.Deps{
		OrgSvc:             service.NewOrganizationService(orgRepo),
		ProjSvc:            service.NewProjectService(projRepo, service.NewSeederService(ruleRepo, skillRepo, cfg.Sandbox.SkillsRoot)),
		RepoSvc:            repoSvc,
		AgentSvc:           service.NewAgentService(agentRepo).WithRoleTemplateRepo(roleTemplateRepo).WithSkillRepo(skillRepo),
		TaskSvc:            service.NewTaskService(taskRepo),
		RuleSvc:            service.NewRuleService(ruleRepo),
		SkillSvc:           service.NewSkillService(skillRepo, cfg.Sandbox.SkillsRoot),
		AnalyticsSvc:       service.NewAnalyticsService(analyticsRepo),
		DashboardSvc:       service.NewAnalyticsDashboardService(dashboardRepo),
		AuditSvc:           auditSvc,
		AuthSvc:            service.NewAuthService(authRepo, cfg.Auth.JWTSecret),
		MemorySvc:          memorySvc,
		LearningSvc:        learningSvc,
		GitAccountSvc:      service.NewGitAccountService(gitAccountRepo),
		ProviderCredSvc:    credentialPoolSvc,
		VirtualKeySvc:      virtualKeySvc,
		ModelRouteSvc:      modelRouteSvc,
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		orch.StartWorkspacePruner(workerCtx)
	}()
	slog.Info("workspace pruner started",
		"retention_hours", cfg.Sandbox.WorkspaceRetentionHours,
		"interval_minutes", cfg.Sandbox.WorkspaceCleanupIntervalMinutes,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		orch.StartLogPruner(workerCtx, cfg.Logging.LocalRetentionDays, cfg.Logging.FileRoot)
	}()
	slog.Info("local log file pruner started",
		"retention_days", cfg.Logging.LocalRetentionDays,
		"file_root", cfg.Logging.FileRoot,
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		aigateway.StartCooldownWorker(workerCtx, credentialPoolSvc, time.Minute)
	}()
	slog.Info("provider credential cooldown worker started")
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

func buildLLMProvider(cfg *config.Config, credentialPool *service.CredentialPoolService, virtualKeys *service.VirtualKeyService, routes *service.ModelRouteService) (llm.Provider, error) {
	var fallback llm.Provider
	var err error

	if cfg.LLM.Provider != "gateway" && cfg.LLM.Provider != "" {
		if cfg.LLM.APIKey != "" || cfg.LLM.LLMAPIKey != "" {
			fallback, err = llm.NewProvider(cfg)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if cfg.LLM.OpenAIAPIKey != "" || cfg.LLM.AnthropicAPIKey != "" || cfg.LLM.GeminiAPIKey != "" {
			fallback, err = llm.NewProvider(cfg)
			if err != nil {
				return nil, err
			}
		}
	}

	if credentialPool != nil {
		return aigateway.NewAIGateway(aigateway.Options{
			FallbackProvider:  fallback,
			CredentialPool:    credentialPool,
			VirtualKeyService: virtualKeys,
			ModelRouteService: routes,
			Config:            cfg,
		}), nil
	}

	return fallback, nil
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
