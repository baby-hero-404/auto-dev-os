package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// pauseForClarification appends a ClarificationRound built from the CLI's
// last output line and pauses the workflow, mirroring the API-native flow's
// clarification_required convention (steps/analyze.go) — reused, not
// reinvented, so the frontend's existing clarification UI keeps working
// unchanged. Shared by cli_analyze.go/cli_spec.go/cli_implement.go since any
// of the 3 CLI spec-first steps can be the one that detects the CLI stopped
// to ask a question (REQ-006, docs/openspecs/cli-execution-reliability).
func pauseForClarification(ctx context.Context, tasks TaskUpdater, task *models.Task, stepID string, cliOutput string) (StepResult, error) {
	var rounds []models.ClarificationRound
	if len(task.Clarifications) > 0 {
		_ = json.Unmarshal(task.Clarifications, &rounds)
	}
	rounds = append(rounds, models.ClarificationRound{
		Round:     len(rounds) + 1,
		Timestamp: time.Now(),
		Questions: []string{lastNonEmptyLine(cliOutput)},
	})
	raw, err := json.Marshal(rounds)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal clarifications: %w", stepID, err)
	}
	specStatus := models.TaskSpecStatusClarificationRequired
	pausedStep := stepID
	if tasks != nil {
		if _, err := tasks.Update(ctx, task.ID, models.UpdateTaskInput{
			Clarifications: raw,
			SpecStatus:     &specStatus,
			PausedStep:     &pausedStep,
		}); err != nil {
			return nil, fmt.Errorf("%s: persist clarification: %w", stepID, err)
		}
	}
	task.Clarifications = raw
	task.SpecStatus = specStatus
	task.PausedStep = pausedStep
	return nil, workflow.PauseError{Step: stepID, Reason: "workflow paused for human task clarification (cli)"}
}

// lastNonEmptyLine returns the last non-blank line of s, trimmed, or "" if
// every line is blank. Duplicated (not imported) from engine/cli.go — the
// steps package deliberately doesn't depend on the engine package (CLI
// dispatch is abstracted behind CLIStepRunner), and this is 8 lines.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}
