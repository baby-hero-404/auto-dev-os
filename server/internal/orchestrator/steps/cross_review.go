package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// CrossReviewStep implements Step for the optional independent AI review pass
// on CLI-coded changes (cross-harness-review REQ-M02, design.md "cross_review
// step (CLI flow)"): reviews the cli_implement diff against the approved spec
// set, resolving the review harness (same/different_model/different_provider)
// against the CLI's declared underlying_provider rather than assuming
// "cli:<command>" is automatically different from every API provider.
type CrossReviewStep struct {
	rt               StepRuntime
	tasks            TaskReader
	projects         ProjectReader
	llm              LLMRunner
	diff             DiffCapturer
	worktree         WorktreeHostPathResolver
	artifacts        ArtifactSaver
	checkpoints      CheckpointReader
	checkpointLister CheckpointLister
	status           StatusUpdater
	log              Logger
}

func NewCrossReviewStep(
	rt StepRuntime,
	tasks TaskReader,
	projects ProjectReader,
	llmRunner LLMRunner,
	diff DiffCapturer,
	worktree WorktreeHostPathResolver,
	artifacts ArtifactSaver,
	checkpoints CheckpointReader,
	checkpointLister CheckpointLister,
	status StatusUpdater,
	log Logger,
) *CrossReviewStep {
	return &CrossReviewStep{
		rt:               rt,
		tasks:            tasks,
		projects:         projects,
		llm:              llmRunner,
		diff:             diff,
		worktree:         worktree,
		artifacts:        artifacts,
		checkpoints:      checkpoints,
		checkpointLister: checkpointLister,
		status:           status,
		log:              log,
	}
}

func (s *CrossReviewStep) ID() string { return workflow.StepCrossReview }

func (s *CrossReviewStep) StatusOnResume(output StepResult) string {
	if limitReached, ok := output["cycle_limit_reached"].(bool); ok && limitReached {
		return models.TaskStatusHumanReview
	}
	if verdictRaw, ok := output["review_verdict"].(map[string]any); ok {
		if spec, ok := verdictRaw["spec_compliance"].(map[string]any); ok {
			if strings.EqualFold(fmt.Sprintf("%v", spec["verdict"]), "fail") {
				return models.TaskStatusCoding
			}
		}
	}
	return models.TaskStatusHumanReview
}

// effectiveCLIHarness resolves the {engine, provider} of the coder side for a
// CLI-coded task: underlying_provider when declared, otherwise "cli:<command>"
// as an opaque harness ID (REQ-001b blind-spot guard).
func effectiveCLIHarness(p *models.Project) (engine string, provider string) {
	if p == nil {
		return models.ExecutionEngineCLI, ""
	}
	var cfg models.CLIEngineConfig
	if len(p.CLIEngineConfig) > 0 {
		_ = json.Unmarshal(p.CLIEngineConfig, &cfg)
	}
	if cfg.UnderlyingProvider != "" {
		return models.ExecutionEngineCLI, cfg.UnderlyingProvider
	}
	return models.ExecutionEngineCLI, "cli:" + cfg.Command
}

// crossReviewFeedback reads the most recent cross_review checkpoint's
// structured verdict and formats its violations/issues as a "## Reviewer
// feedback" block for the next cli_implement re-dispatch (design.md's
// cross_review routing-fail path).
func crossReviewFeedback(ctx context.Context, lister CheckpointLister, taskID string) string {
	if lister == nil {
		return ""
	}
	checkpoints, err := lister.ListCheckpoints(ctx, taskID)
	if err != nil {
		return ""
	}
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != workflow.StepCrossReview {
			continue
		}
		var state map[string]any
		if json.Unmarshal(cp.State, &state) != nil {
			continue
		}
		output, ok := state["output"].(map[string]any)
		if !ok {
			continue
		}
		verdictRaw, ok := output["review_verdict"].(map[string]any)
		if !ok {
			continue
		}
		var verdict models.ReviewVerdict
		if b, err := json.Marshal(verdictRaw); err == nil {
			_ = json.Unmarshal(b, &verdict)
		}
		var sb strings.Builder
		for _, v := range verdict.SpecCompliance.Violations {
			sb.WriteString(fmt.Sprintf("- [spec] %s (observed: %s, severity: %s)\n", v.Requirement, v.Observed, v.Severity))
		}
		for _, iss := range verdict.CodeQuality.Issues {
			sb.WriteString(fmt.Sprintf("- [quality] %s:%d %s (suggestion: %s)\n", iss.File, iss.Line, iss.Issue, iss.Suggestion))
		}
		return sb.String()
	}
	return ""
}

func (s *CrossReviewStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}

	maxCycles := 3
	var project *models.Project
	if s.projects != nil {
		if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
			project = p
			if p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
		}
	}
	reviewCycleCount := 0
	if s.checkpoints != nil {
		reviewCycleCount = s.checkpoints.CountSuccessful(ctx, s.rt.Task.ID, workflow.StepCrossReview)
	}

	var diffText string
	if s.diff != nil {
		diffText, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepCrossReview, "")
		if s.artifacts != nil && diffText != "" {
			_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "diff", diffText)
		}
	}
	if diffText == "" && s.log != nil {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "no diff was provided to cross_review step")
	}

	slug := TaskSpecSlug(s.rt.Task)
	var specsText string
	if s.worktree != nil {
		if root, err := s.worktree.ResolveHostWorktreeRoot(ctx, s.rt.Task); err == nil {
			specDir := filepath.Join(root, "docs", "openspecs", slug)
			for _, name := range []string{"specs.md", "tasks.md"} {
				if b, err := os.ReadFile(filepath.Join(specDir, name)); err == nil {
					specsText += fmt.Sprintf("\n\n### %s\n\n%s", name, string(b))
				}
			}
		}
	}

	instruction := "Review the changes made by another AI coding agent against the approved spec set. " +
		"Here is the current workspace diff:\n\n" + diffText + "\n\n" +
		"Here is the approved spec set the implementation must satisfy:" + specsText + "\n\n" +
		"Return JSON with spec_compliance {verdict, violations[]}, code_quality {verdict, issues[]}, and summary."

	// Resolve review harness per review_harness_policy (REQ-001, REQ-001b).
	policy := models.ReviewHarnessDifferentModel
	if project != nil && project.ReviewHarnessPolicy != "" {
		policy = project.ReviewHarnessPolicy
	}
	coderEngine, coderProvider := effectiveCLIHarness(project)

	adversarial := policy == models.ReviewHarnessSame ||
		previousSelfReviewFallback(ctx, s.checkpointLister, s.rt.Task.ID, workflow.StepCrossReview)

	var routeTrace *llm.RouteTrace
	switch {
	case policy == models.ReviewHarnessSame:
		// no exclusion: reviewer is allowed to be the same harness that coded it.
	case coderProvider != "":
		if s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("cross-harness review: excluding coder provider %s (%s) for review", coderProvider, coderEngine))
		}
		ctx = llm.WithExcludeProviderID(ctx, coderProvider)
		ctx, routeTrace = llm.WithRouteTrace(ctx)
	default:
		if s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "cross-harness review: no underlying_provider declared for the CLI engine; unable to exclude a specific provider")
		}
	}

	if adversarial {
		instruction += adversarialAuditDirective
	}

	out, err := s.llm.RunLLMStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, workflow.StepCrossReview, instruction)
	if err != nil {
		return nil, err
	}
	if routeTrace != nil && routeTrace.SelfReviewFallback {
		if s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("cross-harness review fallback: reviewed by the same harness that wrote the code (%s) — no alternative provider was available", routeTrace.ActualModel))
		}
		out["self_review_fallback"] = true
		out["self_review_fallback_model"] = routeTrace.ActualModel
	}
	out["coded_by"] = map[string]any{"engine": coderEngine, "provider": coderProvider, "model": ""}
	reviewedBy := map[string]any{}
	if m, ok := out["model"].(string); ok && m != "" {
		reviewedBy["model"] = m
	}
	if p, ok := out["provider"].(string); ok && p != "" {
		reviewedBy["provider"] = p
	}
	if len(reviewedBy) > 0 {
		out["reviewed_by"] = reviewedBy
	}

	hasFindings := false
	specFail := false
	var verdict models.ReviewVerdict
	verdictOK := false
	if parsed, ok := out["parsed"].(map[string]any); ok {
		if s.artifacts != nil {
			_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepCrossReview, "review_findings", parsed)
		}
		verdict, verdictOK = ParseReviewVerdict(parsed)
		if verdictOK {
			specFail = strings.EqualFold(strings.TrimSpace(verdict.SpecCompliance.Verdict), "fail")
			qualityFail := strings.EqualFold(strings.TrimSpace(verdict.CodeQuality.Verdict), "fail")
			hasFindings = specFail || qualityFail
			if verdictMap := verdictAsMap(verdict); verdictMap != nil {
				out["review_verdict"] = verdictMap
			}
		}
	}

	if specFail && verdictOK {
		prevViolations := previousReviewViolationsForStep(ctx, s.checkpointLister, s.rt.Task.ID, workflow.StepCrossReview)
		if hasRepeatViolation(prevViolations, verdict.SpecCompliance.Violations) {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "cross-review spec violation repeated across consecutive cycles; escalating for human decision")
			}
			if s.status != nil {
				if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview); err != nil {
					return nil, err
				}
			}
			return out, workflow.PauseError{Step: workflow.StepCrossReview, Reason: "awaiting_review_escalation: same spec violation repeated across consecutive cross-review cycles"}
		}
	}

	if hasFindings && reviewCycleCount >= maxCycles {
		if s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("cross-review cycle limit reached (%d/%d), proceeding to MR despite findings", reviewCycleCount, maxCycles))
		}
		out["cycle_limit_reached"] = true
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview); err != nil {
				return nil, err
			}
		}
		return out, nil
	}

	if hasFindings {
		if verdictMap := verdictAsMap(verdict); verdictMap != nil {
			out["review_verdict"] = verdictMap
		}
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusCoding); err != nil {
				return nil, err
			}
		}
		return out, workflow.ErrCrossReviewFixLoop
	}

	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview); err != nil {
			return nil, err
		}
	}
	return out, nil
}
