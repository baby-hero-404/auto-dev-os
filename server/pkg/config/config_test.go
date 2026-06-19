package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func resetConfig(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "test-secret")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}

func TestLoadValidatesQueueInterval(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("QUEUE_WORKER_INTERVAL_MS", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "QUEUE_WORKER_INTERVAL_MS") {
		t.Fatalf("expected QUEUE_WORKER_INTERVAL_MS error, got %v", err)
	}
}

func TestLoadValidConfig(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Worker.Enabled {
		t.Fatal("expected queue worker enabled by default")
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Fatalf("expected default DB max open conns 50, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.LLM.EmbeddingModel != "text-embedding-3-small" {
		t.Fatalf("expected default embedding model, got %q", cfg.LLM.EmbeddingModel)
	}
	if cfg.Sandbox.WorkspaceRetentionHours != 72 {
		t.Fatalf("expected default workspace retention 72h, got %d", cfg.Sandbox.WorkspaceRetentionHours)
	}
	if cfg.Sandbox.WorkspaceCleanupIntervalMinutes != 60 {
		t.Fatalf("expected default workspace cleanup interval 60m, got %d", cfg.Sandbox.WorkspaceCleanupIntervalMinutes)
	}
	if cfg.Logging.LocalRetentionDays != 14 {
		t.Fatalf("expected default local retention days 14, got %d", cfg.Logging.LocalRetentionDays)
	}
	if cfg.AutoCodeOS.DataRoot != "./.data" {
		t.Fatalf("expected default data_root ./.data, got %q", cfg.AutoCodeOS.DataRoot)
	}
	if cfg.Logging.FileRoot != ".data/logs" {
		t.Fatalf("expected default log file root .data/logs, got %q", cfg.Logging.FileRoot)
	}
	if cfg.Sandbox.WorkspaceRoot != ".data/workspaces" {
		t.Fatalf("expected default workspace root .data/workspaces, got %q", cfg.Sandbox.WorkspaceRoot)
	}
	if cfg.Sandbox.SkillsRoot != ".data/skills" {
		t.Fatalf("expected default skills root .data/skills, got %q", cfg.Sandbox.SkillsRoot)
	}
}

func TestLoadDataRootOverride(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("AUTO_CODE_OS_DATA_ROOT", "/tmp/custom-root")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AutoCodeOS.DataRoot != "/tmp/custom-root" {
		t.Fatalf("expected overridden data_root /tmp/custom-root, got %q", cfg.AutoCodeOS.DataRoot)
	}
	if cfg.Logging.FileRoot != "/tmp/custom-root/logs" {
		t.Fatalf("expected overridden log root /tmp/custom-root/logs, got %q", cfg.Logging.FileRoot)
	}
	if cfg.Sandbox.WorkspaceRoot != "/tmp/custom-root/workspaces" {
		t.Fatalf("expected overridden workspace root, got %q", cfg.Sandbox.WorkspaceRoot)
	}
	if cfg.Sandbox.SkillsRoot != "/tmp/custom-root/skills" {
		t.Fatalf("expected overridden skills root, got %q", cfg.Sandbox.SkillsRoot)
	}
}

func TestLoadDatabasePoolOverrides(t *testing.T) {
	resetConfig(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DB_MAX_OPEN_CONNS", "80")
	t.Setenv("DB_MAX_IDLE_CONNS", "20")
	t.Setenv("DB_CONN_MAX_LIFETIME_SECONDS", "900")
	t.Setenv("DB_CONN_MAX_IDLE_TIME_SECONDS", "120")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Database.MaxOpenConns != 80 || cfg.Database.MaxIdleConns != 20 {
		t.Fatalf("unexpected pool sizes: %+v", cfg.Database)
	}
	if cfg.Database.ConnMaxLifetimeSeconds != 900 || cfg.Database.ConnMaxIdleTimeSeconds != 120 {
		t.Fatalf("unexpected pool durations: %+v", cfg.Database)
	}
}
