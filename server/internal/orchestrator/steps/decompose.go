package steps

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// estimateTokens is a coarse, deterministic heuristic (~4 chars/token,
// matching common tokenizer rules of thumb) used only to feed the
// Complexity Score, not to bill/limit anything precisely.
func estimateTokens(s string) int {
	n := len(strings.TrimSpace(s))
	if n == 0 {
		return 0
	}
	return n/4 + 1
}

// deliverableSeparators splits a task's title+description into independent
// objectives — e.g. "build schema + API + frontend" = 3 deliverables. This
// is intentionally a coarse heuristic (proposal.md): it counts separator-
// joined clauses, not a semantic understanding of the task.
var deliverableSeparators = regexp.MustCompile(`(?i)\s*(?:\+|,| and | then | as well as )\s*`)

// countDeliverables returns the number of independent objectives implied by
// text, minimum 1 for any non-empty text.
func countDeliverables(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	parts := deliverableSeparators.Split(text, -1)
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	return count
}

// dependencyDepth computes the longest chain length through the already-
// computed TaskDAG.DependsOn graph (analyze's execution-unit dependency
// graph), reusing that computation rather than deriving depth separately.
func dependencyDepth(dag []models.TaskDAG) int {
	if len(dag) == 0 {
		return 0
	}
	byID := make(map[string]models.TaskDAG, len(dag))
	for _, n := range dag {
		byID[n.ID] = n
	}
	memo := make(map[string]int, len(dag))
	var depth func(id string, visiting map[string]bool) int
	depth = func(id string, visiting map[string]bool) int {
		if d, ok := memo[id]; ok {
			return d
		}
		if visiting[id] {
			// Cyclic dependency (shouldn't happen post-DAG-validation) — treat
			// as depth 1 rather than recursing forever.
			return 1
		}
		node, ok := byID[id]
		if !ok || len(node.DependsOn) == 0 {
			memo[id] = 1
			return 1
		}
		visiting[id] = true
		max := 0
		for _, dep := range node.DependsOn {
			if d := depth(dep, visiting); d > max {
				max = d
			}
		}
		delete(visiting, id)
		memo[id] = max + 1
		return memo[id]
	}
	maxDepth := 0
	for _, n := range dag {
		if d := depth(n.ID, map[string]bool{}); d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

// ComputeComplexityScore builds the analyze-time, multi-factor split-risk
// score (docs/openspecs/task-subtask-decomposition Scenario: "Analyze step
// computes a Complexity Score"). Deterministic and LLM-independent so it can
// run for every task, including the deriveWorkflowAnalysis fallback path.
func ComputeComplexityScore(task *models.Task, analysis models.TaskAnalysis) models.ComplexityScore {
	tokens := estimateTokens(task.Title + " " + task.Description + " " + analysis.ProposalMD + " " + analysis.SpecsMD)
	files := len(analysis.AffectedFiles)
	depth := dependencyDepth(analysis.Tasks)
	deliverables := countDeliverables(task.Title)
	if deliverables < countDeliverables(task.Description) {
		deliverables = countDeliverables(task.Description)
	}

	// Weights are intentionally simple integer multipliers (config-tunable
	// threshold, not per-factor weights, keeps this readable and testable;
	// design.md flags per-factor weight tuning as a Risk/Mitigation for
	// later, not required for v1).
	total := tokens/1000 + files*5 + depth*10 + deliverables*15

	return models.ComplexityScore{
		Tokens:           tokens,
		Files:            files,
		DependencyDepth:  depth,
		DeliverableCount: deliverables,
		Total:            total,
	}
}

// BuildProposedSplit produces an ordered []ChildTaskSpec once the
// Complexity Score has crossed the threshold and decomposition isn't
// disabled. Splitting is done by deliverable clause (the same heuristic
// countDeliverables uses) — a coarse but deterministic and reviewable
// starting point; the operator can still reorder/edit/merge/drop children
// under decomposition_mode=manual (specs.md).
func BuildProposedSplit(task *models.Task, analysis models.TaskAnalysis) []models.ChildTaskSpec {
	clauses := deliverableSeparators.Split(strings.TrimSpace(task.Description), -1)
	var trimmed []string
	for _, c := range clauses {
		c = strings.TrimSpace(c)
		if c != "" {
			trimmed = append(trimmed, c)
		}
	}
	if len(trimmed) < 2 {
		// Nothing to split on within the description text; fall back to a
		// single-child "no-op" split description-based on affected files,
		// evenly chunked, so a split is still returned when Files/Depth
		// alone crossed the threshold on a terse description.
		return buildFileChunkedSplit(task, analysis)
	}

	specs := make([]models.ChildTaskSpec, 0, len(trimmed))
	var prevSummary *models.ChildTaskSummary
	for i, clause := range trimmed {
		expected := affectedFilesForClause(analysis.AffectedFiles, i, len(trimmed))
		specs = append(specs, models.ChildTaskSpec{
			Title:        fmt.Sprintf("%s — part %d/%d: %s", task.Title, i+1, len(trimmed), truncate(clause, 60)),
			Instructions: clause,
			Contract: models.ChildTaskContract{
				InputPreviousSummary: prevSummary,
				OutputExpected:       expected,
			},
		})
	}
	return specs
}

// buildFileChunkedSplit is the fallback split strategy when the
// description has no clause separators to split on: chunk the analyzed
// affected files into up to 3 roughly-even groups. Never produces a split
// with fewer than 2 children — the caller only invokes this once a split
// was already decided to be worthwhile.
func buildFileChunkedSplit(task *models.Task, analysis models.TaskAnalysis) []models.ChildTaskSpec {
	files := analysis.AffectedFiles
	if len(files) < 2 {
		return nil
	}
	chunkCount := 3
	if len(files) < chunkCount {
		chunkCount = len(files)
	}
	chunkSize := (len(files) + chunkCount - 1) / chunkCount

	specs := make([]models.ChildTaskSpec, 0, chunkCount)
	var prevSummary *models.ChildTaskSummary
	for i := 0; i < chunkCount; i++ {
		start := i * chunkSize
		if start >= len(files) {
			break
		}
		end := start + chunkSize
		if end > len(files) {
			end = len(files)
		}
		var expected []string
		for _, f := range files[start:end] {
			expected = append(expected, f.File)
		}
		specs = append(specs, models.ChildTaskSpec{
			Title:        fmt.Sprintf("%s — part %d/%d", task.Title, i+1, chunkCount),
			Instructions: fmt.Sprintf("%s\n\nFocus this sub-task strictly on: %s", task.Description, strings.Join(expected, ", ")),
			Contract: models.ChildTaskContract{
				InputPreviousSummary: prevSummary,
				OutputExpected:       expected,
			},
		})
	}
	if len(specs) < 2 {
		return nil
	}
	return specs
}

func affectedFilesForClause(files []models.AffectedFile, idx, total int) []string {
	if len(files) == 0 || total == 0 {
		return nil
	}
	chunkSize := (len(files) + total - 1) / total
	start := idx * chunkSize
	if start >= len(files) {
		return nil
	}
	end := start + chunkSize
	if end > len(files) {
		end = len(files)
	}
	var out []string
	for _, f := range files[start:end] {
		out = append(out, f.File)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
