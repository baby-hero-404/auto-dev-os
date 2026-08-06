package steps

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// SortBySequenceIndex returns children ordered by SequenceIndex ascending.
// Children with a nil SequenceIndex (shouldn't occur for decomposition-
// created children, but defensive) sort last, in original order.
func SortBySequenceIndex(children []models.Task) []models.Task {
	ordered := make([]models.Task, len(children))
	copy(ordered, children)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].SequenceIndex, ordered[j].SequenceIndex
		if a == nil && b == nil {
			return false
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a < *b
	})
	return ordered
}

// summarizeChild builds a child's structured ChildTaskSummary from its
// attempts (changed files come from the child's own Analysis.AffectedFiles —
// the same field every task already populates during analyze — never from
// raw CLI transcript). ContractDeviation is set when the child's actual
// AffectedFiles don't cover every file its Contract's OutputExpected named.
func summarizeChild(child models.Task, attempts []models.TaskAttempt) models.ChildTaskSummary {
	summary := models.ChildTaskSummary{TaskID: child.ID}
	if child.SequenceIndex != nil {
		summary.SequenceIndex = *child.SequenceIndex
	}

	var analysis models.TaskAnalysis
	if len(child.Analysis) > 0 {
		_ = json.Unmarshal(child.Analysis, &analysis)
	}
	changed := make(map[string]bool)
	for _, f := range analysis.AffectedFiles {
		changed[f.File] = true
	}
	for f := range changed {
		summary.ChangedFiles = append(summary.ChangedFiles, f)
	}
	sort.Strings(summary.ChangedFiles)

	var totalCost float64
	var totalDurationSeconds int
	for _, a := range attempts {
		if a.CostUSD != nil {
			totalCost += *a.CostUSD
		}
		if a.FinishedAt != nil {
			totalDurationSeconds += int(a.FinishedAt.Sub(a.StartedAt).Seconds())
		}
	}
	summary.CostUSD = totalCost
	summary.DurationSeconds = totalDurationSeconds

	if child.Status == models.TaskStatusMerged {
		summary.OneLineOutcome = fmt.Sprintf("Completed: %s", child.Title)
	} else {
		summary.OneLineOutcome = fmt.Sprintf("Did not complete (status=%s): %s", child.Status, child.Title)
	}

	return summary
}

// contractDeviation checks a child's actual changed files against its
// Contract's OutputExpected (recovered from the parent's ProposedSplit, by
// SequenceIndex — the Contract itself isn't persisted per-child, only on
// the parent's analyze-time proposal). Returns nil when satisfied or when
// there's no Contract to check against (e.g. manually created sub-tasks).
func contractDeviation(summary models.ChildTaskSummary, expected []string) *string {
	if len(expected) == 0 {
		return nil
	}
	changed := make(map[string]bool, len(summary.ChangedFiles))
	for _, f := range summary.ChangedFiles {
		changed[f] = true
	}
	var missing []string
	for _, f := range expected {
		if !changed[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	msg := fmt.Sprintf("child did not touch %d expected file(s): %v", len(missing), missing)
	return &msg
}

// Reduce is the deterministic (non-LLM) aggregation step (Phase 4,
// docs/openspecs/task-subtask-decomposition): union of changed files across
// every child, summed test pass/fail counts, summed cost/duration, and
// per-child ContractDeviation warnings. It consumes only each child's
// structured Task/TaskAttempt rows — never a child's raw CLI stdout/stderr —
// matching the Rules constraint in specs.md.
func Reduce(parent *models.Task, children []models.Task, allAttempts []models.TaskAttempt) models.DecomposedTaskSummary {
	attemptsByTask := make(map[string][]models.TaskAttempt)
	for _, a := range allAttempts {
		attemptsByTask[a.TaskID] = append(attemptsByTask[a.TaskID], a)
	}

	var parentAnalysis models.TaskAnalysis
	if len(parent.Analysis) > 0 {
		_ = json.Unmarshal(parent.Analysis, &parentAnalysis)
	}
	expectedBySeq := make(map[int][]string, len(parentAnalysis.ProposedSplit))
	for i, spec := range parentAnalysis.ProposedSplit {
		expectedBySeq[i] = spec.Contract.OutputExpected
	}

	ordered := SortBySequenceIndex(children)

	summary := models.DecomposedTaskSummary{}
	changedSet := make(map[string]bool)
	for _, child := range ordered {
		childSummary := summarizeChild(child, attemptsByTask[child.ID])
		if expected, ok := expectedBySeq[childSummary.SequenceIndex]; ok {
			childSummary.ContractDeviation = contractDeviation(childSummary, expected)
		}
		summary.Children = append(summary.Children, childSummary)
		for _, f := range childSummary.ChangedFiles {
			changedSet[f] = true
		}
		summary.TestsPassed += childSummary.TestsPassed
		summary.TestsFailed += childSummary.TestsFailed
		summary.CostUSD += childSummary.CostUSD
		summary.DurationSeconds += childSummary.DurationSeconds
	}
	for f := range changedSet {
		summary.ChangedFiles = append(summary.ChangedFiles, f)
	}
	sort.Strings(summary.ChangedFiles)

	if parentAnalysis.ComplexityScore != nil {
		summary.TokensBeforeSplit = parentAnalysis.ComplexityScore.Tokens
	}
	summary.TokensAfterSplit = 0
	for _, a := range allAttempts {
		if a.TokensIn != nil {
			summary.TokensAfterSplit += *a.TokensIn
		}
		if a.TokensOut != nil {
			summary.TokensAfterSplit += *a.TokensOut
		}
	}

	// duration.single_estimate isn't independently modeled in v1 (analyze
	// doesn't estimate wall-clock time for a hypothetical single-turn run);
	// approximated here as the sum of actual child durations, so
	// cost.saved/duration comparisons stay well-defined rather than
	// silently comparing against zero. Flagged for future refinement once
	// analyze produces a real single-turn time estimate.
	summary.DurationSingleEst = summary.DurationSeconds
	summary.CostSavedUSD = 0

	return summary
}

// decomposedSummaryAnalysisKey is embedded into the parent Task's Analysis
// JSON so the aggregate is retrievable from the existing Task.Analysis
// column without a new migration.
const decomposedSummaryAnalysisKey = "decomposed_summary"

// MarshalAnalysisPatch returns existing (a parent Task's Analysis JSON) with
// summary merged in under decomposedSummaryAnalysisKey, preserving every
// other existing field. Exported as a package-level function (rather than a
// method on models.DecomposedTaskSummary, which lives in a different
// package) so the orchestrator package can call it directly after
// steps.Reduce.
func MarshalAnalysisPatch(summary models.DecomposedTaskSummary, existing []byte) ([]byte, error) {
	var doc map[string]any
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			doc = map[string]any{}
		}
	} else {
		doc = map[string]any{}
	}
	doc[decomposedSummaryAnalysisKey] = summary
	return json.Marshal(doc)
}
