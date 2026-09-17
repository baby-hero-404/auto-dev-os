// Review-verdict parsing and cross-cycle comparison helpers. These were
// factored out of review.go when the API-native ReviewStep was removed:
// cross_review is the only remaining consumer, but it needs the whole set —
// verdict parsing, repeat-violation detection across cycles, and the
// adversarial directive.
package steps

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
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
