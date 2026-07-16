package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/auto-code-os/auto-code-os/server/internal/database"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/rollout"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/config"
)

func main() {
	sampleSize := flag.Int("sample", 100, "Number of terminal tasks to sample")
	// threshold's zero-value sentinel (-1) lets us tell "flag not passed" apart from
	// "flag explicitly set to 0" so cfg.Execution.RolloutViolationThresholdPct (default 1.0,
	// see docs/openspecs/runtime-centric-completion-2026 REQ-001) can supply the real default.
	threshold := flag.Float64("threshold", -1, "Violation rate threshold percentage (0.0 to 100.0); defaults to execution.rollout_violation_threshold_pct from config")
	flag.Parse()

	if err := run(*sampleSize, *threshold); err != nil {
		slog.Error("rollout gate evaluation failed", "error", err)
		os.Exit(1)
	}
}

func run(sampleSize int, thresholdPct float64) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if thresholdPct < 0 {
		thresholdPct = cfg.Execution.RolloutViolationThresholdPct
	}

	db, err := database.ConnectWithPool(cfg.Database.URL, database.PoolConfig{
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	})
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}

	taskRepo := repository.NewTaskRepo(db)
	workflowRepo := repository.NewWorkflowRepo(db)
	workflowRepo.SetLogFileRoot(cfg.Logging.FileRoot)

	result, err := rollout.EvaluateStateMachineGate(context.Background(), taskRepo, workflowRepo, sampleSize, thresholdPct)
	if err != nil {
		return fmt.Errorf("evaluate gate: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("encode JSON result: %w", err)
	}

	return nil
}
