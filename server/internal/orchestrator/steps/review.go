package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/governance"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func hasActionableFindings(findings []models.ReviewFinding) bool {
	for _, f := range findings {
		if f.RequiresFix {
			return true
		}
		s := strings.ToLower(strings.TrimSpace(f.Severity))
		if s == "warning" || s == "error" || s == "high" || s == "blocking" || s == "critical" || s == "medium" {
			return true
		}
	}
	return false
}

// ParseReviewVerdict extracts the structured 2-verdict review schema
// ({spec_compliance, code_quality, summary}) from a parsed LLM completion. It
// returns ok=false when neither axis is present, so callers can fall back to
// the legacy single-verdict findings parsing (REQ-001).
func ParseReviewVerdict(parsed map[string]any) (models.ReviewVerdict, bool) {
	var v models.ReviewVerdict
	specRaw, hasSpec := parsed["spec_compliance"].(map[string]any)
	qualRaw, hasQual := parsed["code_quality"].(map[string]any)
	if !hasSpec && !hasQual {
		return v, false
	}
	if hasSpec {
		v.SpecCompliance.Verdict, _ = specRaw["verdict"].(string)
		if viol, ok := specRaw["violations"].([]any); ok {
			for _, item := range viol {
				if m, ok := item.(map[string]any); ok {
					var sv models.SpecViolation
					sv.Requirement, _ = m["requirement"].(string)
					sv.Observed, _ = m["observed"].(string)
					sv.Severity, _ = m["severity"].(string)
					v.SpecCompliance.Violations = append(v.SpecCompliance.Violations, sv)
				}
			}
		}
	}
	if hasQual {
		v.CodeQuality.Verdict, _ = qualRaw["verdict"].(string)
		if issues, ok := qualRaw["issues"].([]any); ok {
			for _, item := range issues {
				if m, ok := item.(map[string]any); ok {
					var qi models.QualityIssue
					qi.File, _ = m["file"].(string)
					switch lv := m["line"].(type) {
					case float64:
						qi.Line = int(lv)
					case int:
						qi.Line = lv
					}
					qi.Issue, _ = m["issue"].(string)
					qi.Suggestion, _ = m["suggestion"].(string)
					v.CodeQuality.Issues = append(v.CodeQuality.Issues, qi)
				}
			}
		}
	}
	v.Summary, _ = parsed["summary"].(string)
	return v, true
}

func verdictAsMap(v models.ReviewVerdict) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func tokenSet(s string) map[string]struct{} {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		set[f] = struct{}{}
	}
	return set
}

// tokenSetOverlap returns the Jaccard similarity of the whitespace-tokenized,
// lowercased words in a and b — used to fuzzy-match a violation's requirement
// text across review cycles despite the reviewer LLM rephrasing it slightly.
func tokenSetOverlap(a, b string) float64 {
	setA, setB := tokenSet(a), tokenSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for t := range setA {
		if _, ok := setB[t]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// hasRepeatViolation reports whether any violation in cur fuzzy-matches
// (token-set overlap >= 0.6) any violation in prev, meaning the same spec
// failure persisted across two consecutive review cycles (REQ-003).
func hasRepeatViolation(prev, cur []models.SpecViolation) bool {
	for _, c := range cur {
		for _, p := range prev {
			if tokenSetOverlap(c.Requirement, p.Requirement) >= 0.6 {
				return true
			}
		}
	}
	return false
}

// previousReviewViolations finds the most recent prior StepReview checkpoint's
// stored spec_compliance violations, for repeat-violation escalation detection.
func previousReviewViolations(ctx context.Context, lister CheckpointLister, taskID string) []models.SpecViolation {
	return previousReviewViolationsForStep(ctx, lister, taskID, workflow.StepReview)
}

func previousReviewViolationsForStep(ctx context.Context, lister CheckpointLister, taskID, stepID string) []models.SpecViolation {
	if lister == nil {
		return nil
	}
	checkpoints, err := lister.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil
	}
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != stepID {
			continue
		}
		var state map[string]any
		if err := json.Unmarshal(cp.State, &state); err != nil {
			continue
		}
		output, _ := state["output"].(map[string]any)
		verdictRaw, ok := output["review_verdict"].(map[string]any)
		if !ok {
			continue
		}
		specRaw, ok := verdictRaw["spec_compliance"].(map[string]any)
		if !ok {
			continue
		}
		violRaw, ok := specRaw["violations"].([]any)
		if !ok {
			continue
		}
		var result []models.SpecViolation
		for _, item := range violRaw {
			if m, ok := item.(map[string]any); ok {
				var sv models.SpecViolation
				sv.Requirement, _ = m["requirement"].(string)
				sv.Observed, _ = m["observed"].(string)
				sv.Severity, _ = m["severity"].(string)
				result = append(result, sv)
			}
		}
		return result
	}
	return nil
}

func ParseReviewFindings(parsed map[string]any) ([]models.ReviewFinding, error) {
	if parsed == nil {
		return nil, nil
	}

	var rawFindings []any

	if findings, exists := parsed["findings"]; exists {
		if slice, ok := findings.([]any); ok {
			rawFindings = slice
		} else if item, ok := findings.(map[string]any); ok {
			rawFindings = []any{item}
		}
	} else if arr, exists := parsed["array"]; exists {
		if slice, ok := arr.([]any); ok {
			rawFindings = slice
		} else if item, ok := arr.(map[string]any); ok {
			rawFindings = []any{item}
		}
	} else if _, hasFile := parsed["file"]; hasFile {
		rawFindings = []any{parsed}
	} else if _, hasRec := parsed["recommendation"]; hasRec {
		rawFindings = []any{parsed}
	}

	var result []models.ReviewFinding
	for _, raw := range rawFindings {
		fMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var f models.ReviewFinding
		if repo, ok := fMap["repo"].(string); ok {
			f.Repo = repo
		}
		if file, ok := fMap["file"].(string); ok {
			f.File = file
		}
		if lineVal, ok := fMap["line"]; ok {
			switch val := lineVal.(type) {
			case float64:
				f.Line = int(val)
			case int:
				f.Line = val
			}
		}
		if severity, ok := fMap["severity"].(string); ok {
			f.Severity = severity
		}
		if reqFix, ok := fMap["requires_fix"].(bool); ok {
			f.RequiresFix = reqFix
		}
		if recommendation, ok := fMap["recommendation"].(string); ok {
			f.Recommendation = recommendation
		} else if rec, ok := fMap["recommendation_text"].(string); ok {
			f.Recommendation = rec
		} else if rec, ok := fMap["message"].(string); ok {
			f.Recommendation = rec
		}
		result = append(result, f)
	}

	return result, nil
}

func getCoderModel(ctx context.Context, lister CheckpointLister, taskID string) string {
	return getCoderField(ctx, lister, taskID, "model")
}

// getCoderProvider returns the provider name of the most recent coding/fix
// step, mirroring getCoderModel (REQ-001, different_provider policy).
func getCoderProvider(ctx context.Context, lister CheckpointLister, taskID string) string {
	return getCoderField(ctx, lister, taskID, "provider")
}

func getCoderField(ctx context.Context, lister CheckpointLister, taskID, field string) string {
	if lister == nil {
		return ""
	}
	checkpoints, err := lister.ListCheckpoints(ctx, taskID)
	if err != nil || len(checkpoints) == 0 {
		return ""
	}
	// Search backwards for the most recent coding or fix step
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == workflow.StepCodeBackend || cp.Step == workflow.StepCodeFrontend || cp.Step == workflow.StepFix {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if output, ok := state["output"].(map[string]any); ok {
					if val, ok := output[field].(string); ok && val != "" {
						return val
					}
				}
				if val, ok := state[field].(string); ok && val != "" {
					return val
				}
			}
		}
	}
	return ""
}

// previousSelfReviewFallback reports whether the most recent checkpoint for
// stepID recorded a Harness Independence self-review fallback (i.e. the
// reviewer ended up being the same model/provider that wrote the code) —
// used to trigger the adversarial audit directive on the next cycle (REQ-001c).
func previousSelfReviewFallback(ctx context.Context, lister CheckpointLister, taskID, stepID string) bool {
	if lister == nil {
		return false
	}
	checkpoints, err := lister.ListCheckpoints(ctx, taskID)
	if err != nil {
		return false
	}
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != stepID {
			continue
		}
		var state map[string]any
		if err := json.Unmarshal(cp.State, &state); err != nil {
			continue
		}
		output, _ := state["output"].(map[string]any)
		flagged, _ := output["self_review_fallback"].(bool)
		return flagged
	}
	return false
}

// adversarialAuditDirective is appended to the review instruction whenever the
// resolved review harness ends up being the same model/provider that wrote the
// code (REQ-001c) — degrading review quality if left unmentioned, since a model
// reviewing its own output shares the same reasoning blind spots.
const adversarialAuditDirective = "\n\n## Adversarial audit mode\n" +
	"You are auditing code generated by another AI system. Do not assume its logic is\n" +
	"correct or that its tests are sufficient. Actively hunt for: off-by-one and boundary\n" +
	"errors, swallowed errors / ignored returns, nil/undefined dereference, injection and\n" +
	"unsafe input handling, race conditions, and spec requirements silently skipped.\n"

// ReviewStep implements Step for the automated review phase.
type ReviewStep struct {
	rt               StepRuntime
	tasks            TaskReader
	projects         ProjectReader
	llm              LLMRunner
	diff             DiffCapturer
	artifacts        ArtifactSaver
	assigner         ReviewerAssigner
	checkpoints      CheckpointReader
	checkpointLister CheckpointLister
	status           StatusUpdater
	log              Logger
}

func NewReviewStep(
	rt StepRuntime,
	tasks TaskReader,
	projects ProjectReader,
	llm LLMRunner,
	diff DiffCapturer,
	artifacts ArtifactSaver,
	assigner ReviewerAssigner,
	checkpoints CheckpointReader,
	checkpointLister CheckpointLister,
	status StatusUpdater,
	log Logger,
) *ReviewStep {
	return &ReviewStep{
		rt:               rt,
		tasks:            tasks,
		projects:         projects,
		llm:              llm,
		diff:             diff,
		artifacts:        artifacts,
		assigner:         assigner,
		checkpoints:      checkpoints,
		checkpointLister: checkpointLister,
		status:           status,
		log:              log,
	}
}

func (s *ReviewStep) ID() string { return workflow.StepReview }

func (s *ReviewStep) StatusOnResume(output StepResult) string {
	if limitReached, ok := output["cycle_limit_reached"].(bool); ok && limitReached {
		return models.TaskStatusTesting
	}
	nextStatus := models.TaskStatusTesting
	if verdictRaw, ok := output["review_verdict"].(map[string]any); ok {
		specFail := false
		qualityFail := false
		if spec, ok := verdictRaw["spec_compliance"].(map[string]any); ok {
			specFail = strings.EqualFold(fmt.Sprintf("%v", spec["verdict"]), "fail")
		}
		if qual, ok := verdictRaw["code_quality"].(map[string]any); ok {
			qualityFail = strings.EqualFold(fmt.Sprintf("%v", qual["verdict"]), "fail")
		}
		if specFail || qualityFail {
			nextStatus = models.TaskStatusFixing
		}
		return nextStatus
	}
	if parsed, ok := output["parsed"].(map[string]any); ok {
		findings, _ := ParseReviewFindings(parsed)
		if len(findings) > 0 && hasActionableFindings(findings) {
			nextStatus = models.TaskStatusFixing
		}
	}
	return nextStatus
}

func (s *ReviewStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return StepResult{"status": "skipped", "info": "skipped review step for easy task"}, nil
	}
	if s.projects != nil {
		if p, perr := s.projects.GetByID(ctx, s.rt.Task.ProjectID); perr == nil && len(p.PipelineConfig) > 0 {
			if cfg, _, _ := governance.ValidateConfig(p.PipelineConfig); cfg.ShouldSkipStepForLabels(workflow.StepReview, s.rt.Task.Labels) {
				return StepResult{"status": "skipped", "info": "skipped review step via pipeline_config skip_when"}, nil
			}
		}
	}
	if s.rt.Task.Status == models.TaskStatusFixing || s.rt.Task.Status == models.TaskStatusTesting {
		return StepResult{"status": "bypassed_via_human_review"}, nil
	}
	if s.rt.Task.Status == models.TaskStatusHumanReview {
		return nil, workflow.ErrWaitingApproval
	}
	reviewerAgent := s.rt.Agent
	assignedAgentID := ""
	if s.assigner != nil {
		rev, err := s.assigner.AssignReviewer(ctx, s.rt.Task)
		if rev != nil {
			reviewerAgent = rev
			assignedAgentID = rev.ID
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned reviewer agent %s for review step", reviewerAgent.Name))
			}
		}
		if err != nil {
			if assignedAgentID != "" {
				if releaser, ok := s.assigner.(AgentReleaser); ok {
					_ = releaser.Release(context.WithoutCancel(ctx), assignedAgentID)
				}
			}
		}
	}
	if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
		defer func() {
			if releaser, ok := s.assigner.(AgentReleaser); ok {
				if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
					if s.log != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release reviewer agent failed: %v", err))
					}
				}
			}
		}()
	}

	// Enforce review-fix cycle limit.
	maxCycles := 3
	if s.projects != nil {
		if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
			if p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
			if p.PipelineConfig != nil {
				if cfg, _, _ := governance.ValidateConfig(p.PipelineConfig); cfg != nil {
					if override, ok := cfg.MaxReviewFixCyclesOverride(); ok {
						maxCycles = override
					}
				}
			}
			if p.AutoReviewPolicy == "human_only" {
				if s.rt.Task.Status != models.TaskStatusHumanReview {
					if s.status != nil {
						_, _ = s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview)
					}
				}
				return nil, workflow.ErrWaitingApproval
			}
		}
	}
	reviewCycleCount := 0
	if s.checkpoints != nil {
		reviewCycleCount = s.checkpoints.CountSuccessful(ctx, s.rt.Task.ID, workflow.StepReview)
	}
	if s.llm != nil {
		var diffText string
		if s.diff != nil {
			var err error
			diffText, err = s.diff.CapturePRDiff(ctx, s.rt.Task, s.rt.Agent, "main")
			if err != nil || diffText == "" {
				suffix := ""
				if s.rt.Agent != nil {
					if s.rt.Agent.Role == models.AgentRoleBackend {
						suffix = "-be-worktree"
					} else if s.rt.Agent.Role == models.AgentRoleFrontend {
						suffix = "-fe-worktree"
					}
				}
				diffText, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepReview, suffix)
			}
			if s.artifacts != nil && diffText != "" {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "diff", diffText)
			}
		}
		if diffText == "" && s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "no diff was provided to review step")
		}
		instruction := "Review the proposed changes. Here is the current workspace diff:\n\n" + diffText + "\n\n" +
			"Return JSON findings with severity, file, line, and recommendation."
		var analysis models.TaskAnalysis
		if len(s.rt.Task.Analysis) > 0 {
			_ = json.Unmarshal(s.rt.Task.Analysis, &analysis)
		}
		frozen := LoadFrozenContext(stepCtx, &analysis)

		if frozen != nil {
			if len(frozen.ExecutionUnits) > 0 {
				var objectives []string
				for _, unit := range frozen.ExecutionUnits {
					if unit.Objective != "" {
						objectives = append(objectives, fmt.Sprintf("- Node %s: %s", unit.ID, unit.Objective))
					}
				}
				if len(objectives) > 0 {
					instruction += "\n\nEXECUTION OBJECTIVES - You MUST verify the implementation matches these node objectives:\n" + strings.Join(objectives, "\n")
				}
			}
			if frozen.TasksMD != "" {
				instruction += "\n\nCRITICAL RUBRIC - You MUST verify that the following tasks have been completed correctly and accurately based on the diff. If any task is incomplete or not addressed, report a finding with 'requires_fix': true:\n\n" + frozen.TasksMD
			}
			if len(frozen.AcceptanceCriteria) > 0 {
				acJSON, _ := json.MarshalIndent(frozen.AcceptanceCriteria, "", "  ")
				instruction += "\n\nACCEPTANCE CRITERIA - You MUST verify the code meets these criteria:\n```json\n" + string(acJSON) + "\n```"
			}
			if len(frozen.ExecutionBoundaries) > 0 {
				ebJSON, _ := json.MarshalIndent(frozen.ExecutionBoundaries, "", "  ")
				instruction += "\n\nEXECUTION BOUNDARIES - You MUST verify the code modifications did not violate these boundaries:\n```json\n" + string(ebJSON) + "\n```"
			}
		}

		// Harness Independence: exclude the model/provider used in the preceding
		// coding step per the project's review_harness_policy (REQ-001, REQ-M01).
		policy := models.ReviewHarnessDifferentModel
		if s.projects != nil {
			if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
				if p.ReviewHarnessPolicy != "" {
					policy = p.ReviewHarnessPolicy
				}
				if p.PipelineConfig != nil {
					if cfg, _, _ := governance.ValidateConfig(p.PipelineConfig); cfg != nil {
						if override, ok := cfg.ReviewHarnessOverride(); ok {
							policy = override
						}
					}
				}
			}
		}
		coderModel := getCoderModel(ctx, s.checkpointLister, s.rt.Task.ID)
		coderProvider := getCoderProvider(ctx, s.checkpointLister, s.rt.Task.ID)

		adversarial := policy == models.ReviewHarnessSame ||
			previousSelfReviewFallback(ctx, s.checkpointLister, s.rt.Task.ID, workflow.StepReview)

		var routeTrace *llm.RouteTrace
		switch {
		case policy == models.ReviewHarnessSame:
			// no exclusion: reviewer is allowed to be the same harness that coded it.
		case policy == models.ReviewHarnessDifferentProvider && coderProvider != "":
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("harness independence: excluding coder provider %s for review", coderProvider))
			}
			ctx = llm.WithExcludeProviderID(ctx, coderProvider)
			ctx, routeTrace = llm.WithRouteTrace(ctx)
		case coderModel != "":
			if policy == models.ReviewHarnessDifferentProvider && s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "review_harness_policy=different_provider but no coder provider was recorded; falling back to model-level exclusion")
			}
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("harness independence: excluding coder model %s for review", coderModel))
			}
			ctx = llm.WithExcludeModelID(ctx, coderModel)
			ctx, routeTrace = llm.WithRouteTrace(ctx)
		}

		if adversarial {
			instruction += adversarialAuditDirective
		}

		out, err := s.llm.RunLLMStep(ctx, s.rt.Task, reviewerAgent, s.rt.JobID, workflow.StepReview, instruction)
		if err != nil {
			return nil, err
		}
		if routeTrace != nil && routeTrace.SelfReviewFallback {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("harness independence fallback: reviewed by the same model that wrote the code (%s) — no alternative model was available", routeTrace.ActualModel))
			}
			out["self_review_fallback"] = true
			out["self_review_fallback_model"] = routeTrace.ActualModel
		}
		if coderModel != "" || coderProvider != "" {
			out["coded_by"] = map[string]any{"engine": models.ExecutionEngineAPINative, "provider": coderProvider, "model": coderModel}
		}
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
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepReview, "review_findings", parsed)
			}
			verdict, verdictOK = ParseReviewVerdict(parsed)
			if verdictOK {
				specFail = strings.EqualFold(strings.TrimSpace(verdict.SpecCompliance.Verdict), "fail")
				qualityFail := strings.EqualFold(strings.TrimSpace(verdict.CodeQuality.Verdict), "fail")
				hasFindings = specFail || qualityFail
				if verdictMap := verdictAsMap(verdict); verdictMap != nil {
					out["review_verdict"] = verdictMap
				}
			} else {
				if s.log != nil {
					s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "review output did not match the structured 2-verdict schema; falling back to legacy single-verdict parsing")
				}
				findings, _ := ParseReviewFindings(parsed)
				if len(findings) > 0 && hasActionableFindings(findings) {
					hasFindings = true
				}
			}
		}
		if specFail && verdictOK {
			prevViolations := previousReviewViolations(ctx, s.checkpointLister, s.rt.Task.ID)
			if hasRepeatViolation(prevViolations, verdict.SpecCompliance.Violations) {
				if s.log != nil {
					s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "spec violation repeated across consecutive review cycles; escalating for human decision")
				}
				if s.status != nil {
					if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview); err != nil {
						return nil, err
					}
				}
				return out, workflow.PauseError{Step: workflow.StepReview, Reason: "awaiting_review_escalation: same spec violation repeated across consecutive review cycles"}
			}
		}
		nextStatus := models.TaskStatusFixing
		if !hasFindings {
			nextStatus = models.TaskStatusTesting
		}
		// If we've exceeded the cycle limit, skip fix and proceed to test.
		if hasFindings && reviewCycleCount >= maxCycles {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("review-fix cycle limit reached (%d/%d), proceeding to test despite findings", reviewCycleCount, maxCycles))
			}
			nextStatus = models.TaskStatusTesting
			out["cycle_limit_reached"] = true
		}
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, nextStatus); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
