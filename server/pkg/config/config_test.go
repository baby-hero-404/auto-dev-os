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
}
