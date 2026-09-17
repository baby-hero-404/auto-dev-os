package policy

import (
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// MaxClarificationRounds caps how many times the clarification-required gate
// can pause a task on the same unanswered questions before it gives up and
// lets the pipeline proceed anyway — a task whose author won't (or can't)
// answer clarifications must not deadlock in spec_review forever.
const MaxClarificationRounds = 2

// IsDefinitionOfReadyBypassed reports whether the definition-of-ready gate's
// clarification-required block should be skipped for this task: either the
// task is flagged as a hotfix (governance intentionally trusts hotfixes to
// skip friction), or the task has already cycled through
// MaxClarificationRounds of unanswered/still-insufficient clarifications and
// blocking further would just deadlock the pipeline.
func IsDefinitionOfReadyBypassed(labels []string, priorClarificationRounds int) bool {
	if priorClarificationRounds >= MaxClarificationRounds {
		return true
	}
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), "hotfix") {
			return true
		}
	}
	return false
}

// RequiresSpecReview reports whether the run must pause at spec_review after
// cli_spec, so a human approves the plan before any code is written.
//
// This replaces a six-input matrix (project review policy x agent autonomy x
// project autonomy x high-risk-domain pattern match x complexity x
// clarifications). The matrix optimized for gating risky work automatically,
// but its risk detection was substring matching over file paths — "role"
// matched role_templates.go, "api/" matched every handler — so it fired on
// ordinary work and got ignored, while nobody could predict from a project's
// settings whether a given task would stop and wait. A gate nobody can
// predict is a gate nobody relies on.
//
// The default is now false: a task runs straight through to an opened merge
// request. The enforced human boundary is the MR itself, which nothing merges
// without, and which now carries the OpenSpec the work was based on. An org
// that wants pre-implementation review back sets the flag.
//
// Resolution: the task-level override decides whenever it is set (including
// an explicit false on a project that defaults to true); otherwise the
// project default decides.
func RequiresSpecReview(task *models.Task, project *models.Project) bool {
	if task != nil && task.RequireSpecReview != nil {
		return *task.RequireSpecReview
	}
	if project != nil {
		return project.RequireSpecReview
	}
	return false
}

// ResolveSpecGate returns the (specStatus, taskStatus) cli_spec should write,
// folding in the clarification path so the whole decision lives in one place.
//
// Clarifications gate independently of the flag: when the agent came back with
// unanswered questions, proceeding would silently answer them by assumption,
// which is exactly the failure the questions exist to prevent. priorRounds and
// labels still bound that via IsDefinitionOfReadyBypassed, so an unresponsive
// author cannot deadlock the task forever.
func ResolveSpecGate(task *models.Task, project *models.Project, hasClarifications bool, priorRounds int) (specStatus, taskStatus string) {
	var labels []string
	if task != nil {
		labels = task.Labels
	}
	if hasClarifications && IsDefinitionOfReadyBypassed(labels, priorRounds) {
		// Bypassed rather than silently auto-approved: the caller surfaces the
		// unanswered questions as warnings instead of pretending they were
		// resolved.
		return models.TaskSpecStatusReadyWithWarnings, models.TaskStatusCoding
	}
	if hasClarifications {
		return models.TaskSpecStatusClarificationRequired, models.TaskStatusSpecReview
	}
	if RequiresSpecReview(task, project) {
		return models.TaskSpecStatusPendingReview, models.TaskStatusSpecReview
	}
	return models.TaskSpecStatusAutoApproved, models.TaskStatusCoding
}
