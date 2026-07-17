package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/database"
	aigateway "github.com/auto-code-os/auto-code-os/server/internal/gateway"
	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/handler"
	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/learning"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
	if err := agentRepo.ResetAllStatuses(context.Background()); err != nil {
		slog.Warn("failed to reset stuck agent statuses on startup", "error", err)
	}
	roleTemplateRepo := repository.NewRoleTemplateRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	ruleRepo := repository.NewRuleRepo(db)
	skillRepo := repository.NewSkillRepo(db)
	skillSourceRepo := repository.NewSkillSourceRepo(db)
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
	providerModelRepo := repository.NewProviderModelRepo(db)

	secretCipher, err := service.NewSecretCipher(cfg.Auth.JWTSecret)
	if err != nil {
		return err
	}
	if _, err := service.NewSecretService(secretRepo, cfg.Auth.JWTSecret); err != nil {
		return err
	}
	auditSvc := service.NewAuditService(auditRepo)
	providerModelSvc := service.NewProviderModelService(providerModelRepo)
	credentialPoolSvc := service.NewCredentialPoolService(providerCredentialRepo, secretCipher).WithAudit(auditSvc).WithProviderModelSeeder(providerModelSvc)
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
	promptsRoot := "internal/prompts"
	pathRegistry := paths.NewRegistry(
		cfg.AutoCodeOS.DataRoot,
		cfg.Sandbox.WorkspaceRoot,
		cfg.Sandbox.SkillsRoot,
		cfg.Logging.FileRoot,
		promptsRoot,
		"migration",
	)

	skillSvc := service.NewSkillService(skillRepo, skillSourceRepo, pathRegistry.Skill, pathRegistry.FS)
	agentManager := orchestrator.NewAgentManager(agentRepo)

	ctxEngine, err := provider.NewProvider(cfg.Sandbox.WorkspaceRoot, ":memory:")
	if err != nil {
		return fmt.Errorf("init context engine: %w", err)
	}
	defer ctxEngine.Close()

	promptAssembler := prompts.NewPromptAssemblerWithRules(
		ruleRepo,
		nil,
		pathRegistry.Prompt,
		pathRegistry.FS,
		ctxEngine,
	).WithSkillLister(skillSvc).WithDataRoot(cfg.AutoCodeOS.DataRoot)
	artifactRepo := repository.NewArtifactRepo(db)
	gitProvider := gitops.NewGitHubProvider("")
	gitOpsAdapter := gitops.NewGitOpsAdapter(gitProvider, repoRepo, cfg.Sandbox.WorkspaceRoot, secretCipher)
	gitOpsAdapter.SetGitAccountLookup(gitAccountRepo)

	opts := []orchestrator.Option{
		orchestrator.WithPrompts(promptAssembler),
		orchestrator.WithArtifactRepository(artifactRepo),
		orchestrator.WithGitOpsClient(gitOpsAdapter),
		orchestrator.WithRepositoryRepository(repoRepo),
		orchestrator.WithProjectRepository(projRepo),
		orchestrator.WithWorkspaceRoot(cfg.Sandbox.WorkspaceRoot),
		orchestrator.WithDataRoot(cfg.AutoCodeOS.DataRoot),
		orchestrator.WithWorkspaceRetention(
			time.Duration(cfg.Sandbox.WorkspaceRetentionHours)*time.Hour,
			time.Duration(cfg.Sandbox.WorkspaceCleanupIntervalMinutes)*time.Minute,
		),
		orchestrator.WithLLMTraceLogging(cfg.Logging.LLMTraceEnabled, cfg.Logging.LLMLogLevel),
		orchestrator.WithContextEngine(ctxEngine),
		orchestrator.WithGitConfig(cfg.Git),
		orchestrator.WithDisableNetworking(cfg.Sandbox.DisableNetworking),
	}

	if provider, err := buildLLMProvider(cfg, credentialPoolSvc, providerModelSvc, analyticsRepo); err != nil {
		slog.Warn("llm provider disabled", "error", err)
	} else if provider != nil {
		opts = append(opts, orchestrator.WithLLMProvider(provider))
	}

	// Phase 6: Memory & Learning
	memorySvc := service.NewMemoryService(memoryRepo, edgeRepo)
	hasOpenAIKey := cfg.LLM.OpenAIAPIKey != "" && !strings.Contains(cfg.LLM.OpenAIAPIKey, "your-openai-key")
	if hasOpenAIKey {
		memorySvc.SetEmbedder(llm.NewOpenAIEmbedder(cfg.LLM.OpenAIAPIKey, cfg.LLM.EmbeddingModel))
	} else {
		slog.Warn("memory embeddings disabled: OPENAI_API_KEY is not configured or is a placeholder")
	}
	learningSvc := service.NewLearningService(suggestionRepo, ruleRepo)
	learningSvc.SetSkillService(skillSvc)
	learningSvc.SetPromptRoot(filepath.Clean(promptsRoot))
	memoryHooks := learning.NewMemoryHooks(memorySvc)
	learningEngine := learning.NewLearningEngine(memorySvc, learningSvc, taskRepo)

	opts = append(opts, orchestrator.WithMemoryHooks(memoryHooks), orchestrator.WithLearningEngine(learningEngine))
	opts = append(opts, orchestrator.WithMaxPhaseCost(cfg.Worker.MaxPhaseCost))
	opts = append(opts, orchestrator.WithStateMachineEnabled(cfg.Execution.StateMachineEnabled))
	opts = append(opts, orchestrator.WithMaxToolResultChars(cfg.Execution.MaxToolResultChars))
	opts = append(opts, orchestrator.WithMaxToolIterations(cfg.Execution.MaxToolIterations))

	orch := orchestrator.New(taskRepo, workflowRepo, agentManager, sandboxRuntime, opts...)

	repoSvc := service.NewRepositoryService(repoRepo, secretCipher)
	repoSvc.SetProjectRepo(projRepo)
	repoSvc.SetGitAccountRepo(gitAccountRepo)

	deps := handler.Deps{
		OrgSvc:             service.NewOrganizationService(orgRepo),
		ProjSvc:            service.NewProjectService(projRepo, service.NewSeederService(ruleRepo), cfg.AutoCodeOS.DataRoot),
		RepoSvc:            repoSvc,
		AgentSvc:           service.NewAgentService(agentRepo).WithRoleTemplateRepo(roleTemplateRepo),
		TaskSvc:            service.NewTaskService(taskRepo, projRepo, repoRepo),
		RuleSvc:            service.NewRuleService(ruleRepo),
		SkillSvc:           skillSvc,
		AnalyticsSvc:       service.NewAnalyticsService(analyticsRepo),
		DashboardSvc:       service.NewAnalyticsDashboardService(dashboardRepo),
		AuditSvc:           auditSvc,
		AuthSvc:            service.NewAuthService(authRepo, cfg.Auth.JWTSecret),
		MemorySvc:          memorySvc,
		LearningSvc:        learningSvc,
		GitAccountSvc:      service.NewGitAccountService(gitAccountRepo, secretCipher),
		ProviderCredSvc:    credentialPoolSvc,
		ProviderModelSvc:   providerModelSvc,
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
	defer stopWorker()
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
		orch.StartGlobalCachePrewarmer(workerCtx)
	}()
	slog.Info("global cache prewarmer started", "interval_minutes", 15)

	wg.Add(1)
	go func() {
		defer wg.Done()
		orch.StartCacheGarbageCollector(workerCtx)
	}()
	slog.Info("cache garbage collector started", "interval_minutes", 15)
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
	wg.Add(1)
	go func() {
		defer wg.Done()
		orchestrator.StartAgentWatchdog(workerCtx, agentRepo, 5*time.Minute, 30*time.Minute)
	}()
	slog.Info("agent watchdog worker started")
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

func buildLLMProvider(cfg *config.Config, credentialPool *service.CredentialPoolService, providerModels *service.ProviderModelService, recorder llm.UsageRecorder) (llm.Provider, error) {
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
			FallbackProvider:      fallback,
			CredentialPool:        credentialPool,
			ProviderModelResolver: providerModels,
			Config:                cfg,
			Recorder:              recorder,
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
			DisableNetworking: cfg.Sandbox.DisableNetworking,
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
