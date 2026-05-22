package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/database"
	"github.com/auto-code-os/auto-code-os/server/internal/handler"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
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

	// Run migrations
	migrationsPath, _ := filepath.Abs("migration")
	if err := database.Migrate(cfg.DatabaseURL, migrationsPath); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
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

	deps := handler.Deps{
		OrgSvc:   service.NewOrganizationService(orgRepo),
		ProjSvc:  service.NewProjectService(projRepo),
		RepoSvc:  service.NewRepositoryService(repoRepo),
		AgentSvc: service.NewAgentService(agentRepo),
		TaskSvc:  service.NewTaskService(taskRepo),
		RuleSvc:  service.NewRuleService(ruleRepo),
		SkillSvc: service.NewSkillService(skillRepo),
	}

	router := handler.NewRouter(deps)

	// HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server starting", "port", cfg.ServerPort)
		errCh <- srv.ListenAndServe()
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
	return srv.Shutdown(ctx)
}
