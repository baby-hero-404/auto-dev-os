package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// cliCooldownDuration is how long a CLI credential is cooled down for after
// its output matches a quota/rate-limit signature (REQ-006 write-side).
// Matches gateway.go's capped transient-error cooldown for consistency; not
// configurable in this phase.
const cliCooldownDuration = 5 * time.Minute

// finishCLIRun performs the bookkeeping for the code_backend/code_frontend/
// fix dispatch path once RunCodeStep returns without a transport-level
// error: quota-cooldown write-back, auth-failure write-back, the finish log
// line, and saving the cli_output checkpoint artifact. Called by
// finishCLIRun runs immediately after RunCodeStep succeeds, which
// then applies its own noop-check afterward. cliStepRunner.RunCLIStep
// (cli_analyze/cli_spec/cli_implement) inlines the equivalent logic itself
// instead (see cli_spec_step.go) since it needs differentiated success/
// failure log levels (REQ-001) that this shared helper doesn't provide.
func (o *Orchestrator) finishCLIRun(ctx context.Context, taskID, jobID, stepID, credID, instruction string, res *engine.CodeStepResult) {
	// Write-side of REQ-006: only affects the *next* ResolveExecutionProvider
	// call, not this step's own outcome (REQ-005, no mid-task switch).
	if res.QuotaExceeded && credID != "" && o.cooldownSetter != nil {
		cd := cliCooldownDuration
		if res.QuotaCooldown > 0 {
			cd = res.QuotaCooldown
		}
		_ = o.cooldownSetter.SetCooldown(ctx, credID, "", time.Now().Add(cd))
	}

	// Auth-failure write-side: a bad session/token won't self-resolve like a
	// quota cooldown does, so mark the credential out instead of cooling it
	// down (see engine.CodeStepResult.AuthInvalidConfirmed, cli_auth.go).
	// Gated on AuthInvalidConfirmed, not the broader AuthInvalid: a merely
	// suspected match (generic fallback rules only) isn't reliable enough
	// to disable a possibly-still-good credential on — see cli_spec_step.go's
	// mirrored block for the same reasoning.
	if res.AuthInvalidConfirmed && credID != "" && o.credStatusSetter != nil {
		_ = o.credStatusSetter.MarkNeedsReauth(ctx, credID)
	} else if res.AuthInvalid {
		o.log(ctx, taskID, &jobID, "warn", fmt.Sprintf(
			"%s: suspected auth-invalid match on generic fallback rules only (not profile-specific) — credential left active, will retry normally; add a profile-specific rule to engine/cli_auth.go if this recurs",
			stepID))
	}

	o.log(ctx, taskID, &jobID, "info", fmt.Sprintf(
		"%s: cli engine finished (success=%v, exit_code=%d, output_bytes=%d)",
		stepID, res.Success, res.ExitCode, len(res.Output),
	))

	o.initCheckpoints()
	artifactBody := res.Output
	if artifactBody == "" {
		artifactBody = fmt.Sprintf("(cli produced no stdout/stderr; exit_code=%d)\ncommand: %s", res.ExitCode, res.Command)
	}
	_ = o.checkpoints.SaveArtifact(ctx, jobID, taskID, stepID, "cli_output", artifactBody)
	_ = o.checkpoints.SaveArtifact(ctx, jobID, taskID, stepID, "cli_prompt", buildCLIPromptArtifact(res.Command, instruction, nil))
	// Only save a resumable session ID when the run actually ended via a
	// mid-run kill/idle-timeout (REQ-003): resuming into a conversation that
	// failed for another reason (auth, logic error) can make the model repeat
	// the same mistake, so a retry after those failures must start fresh.
	if res.SessionID != "" && (res.IdleTimeoutHit || res.LoopKilled) {
		_ = o.checkpoints.SaveArtifact(ctx, jobID, taskID, stepID, "cli_session_id", res.SessionID)
	}
}

// buildCLIPromptArtifact formats the exact input the CLI agent received —
// the resolved shell invocation, the full instruction text (what
// engine/cli.go writes to .autocode/prompt.md, which is wiped when the
// workspace is cleaned up post-task), and any materialized context files
// (skills/rules injected via PromptBuilder.MaterializeCLIContext) — so it
// can be inspected after the fact the same way the API path's "prompt"
// artifact lets you verify what was actually sent, not just what came back.
func buildCLIPromptArtifact(command, instruction string, contextFiles map[string]string) string {
	var b strings.Builder
	b.WriteString("=== Command ===\n")
	b.WriteString(command)
	b.WriteString("\n\n=== Instruction (prompt.md) ===\n")
	b.WriteString(instruction)
	if len(contextFiles) > 0 {
		paths := make([]string, 0, len(contextFiles))
		for p := range contextFiles {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			b.WriteString(fmt.Sprintf("\n\n=== Context File: %s ===\n", p))
			b.WriteString(contextFiles[p])
		}
	}
	return b.String()
}

func worktreeSuffixForRole(role string) string {
	switch role {
	case models.AgentRoleBackend:
		return models.WorktreeSuffixBackend
	case models.AgentRoleFrontend:
		return models.WorktreeSuffixFrontend
	default:
		return ""
	}
}
