package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/database"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
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
	slog.Info("running migrations...", "path", migrationsPath)
	if err := database.Migrate(cfg.Database.URL, migrationsPath); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	slog.Info("migrations completed successfully")
	return nil
}
