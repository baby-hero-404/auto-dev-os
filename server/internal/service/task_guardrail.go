package service

import (
	"strings"
	"time"
)

// This file implements the execution guardrails from docs/openspecs/
// status-driven-agent-workspace/tasks.md 2.3: the mechanism that makes
// "no per-step human approval" safe. Each Eval* function is pure so it can
// be unit tested independently of the orchestrator's status-transition
// wiring (internal/orchestrator/tracker.go's updateTaskStatus, which calls
// these at the single choke point every status transition passes through)
// and of TaskEventService.Emit (which calls EvalEventVolume before every
// insert).

// GuardrailReason values mirror the task.error event's "reason" field.
const (
	GuardrailReasonMaxRetries     = "max_retries_exceeded"
	GuardrailReasonExecutionTime  = "execution_timeout"
	GuardrailReasonCostBudget     = "cost_budget_exceeded"
	GuardrailReasonEventVolume    = "event_volume_exceeded"
	GuardrailReasonSecurityReview = "security_review_required"
)

// EvalMaxRetries reports whether retryCount has reached maxRetries. A
// maxRetries <= 0 disables the guardrail (treated as "not configured").
func EvalMaxRetries(retryCount, maxRetries int) bool {
	if maxRetries <= 0 {
		return false
	}
	return retryCount >= maxRetries
}

// EvalExecutionTimeout reports whether now is more than maxMinutes past
// startedAt. A nil startedAt or maxMinutes <= 0 disables the guardrail.
func EvalExecutionTimeout(startedAt *time.Time, now time.Time, maxMinutes int) bool {
	if startedAt == nil || maxMinutes <= 0 {
		return false
	}
	return now.Sub(*startedAt) > time.Duration(maxMinutes)*time.Minute
}

// EvalCostBudget always reports false: no execution engine in this codebase
// currently exposes token/cost usage data, so this guardrail is inactive
// rather than enforced against a fabricated estimate (tasks.md 2.3 — "do
// not substitute a conservative estimated limit"). costBudget/costSoFar are
// accepted so call sites don't need special-casing, and are intentionally
// unused until real cost data exists.
func EvalCostBudget(costSoFar *float64, costBudget *float64) bool {
	return false
}

// EvalEventVolume reports whether eventCount has reached maxEvents. A
// maxEvents <= 0 disables the guardrail.
func EvalEventVolume(eventCount, maxEvents int) bool {
	if maxEvents <= 0 {
		return false
	}
	return eventCount >= maxEvents
}

// EvalSecurityReview reports whether a diff should be blocked for human
// review: either it touches a deny-listed path, or its content matches a
// common hardcoded-secret pattern. Returns the tripped reason string (for
// logging) alongside the boolean.
func EvalSecurityReview(changedPaths []string, denyListPaths []string, diffContent string) (bool, string) {
	for _, path := range changedPaths {
		for _, deny := range denyListPaths {
			if deny == "" {
				continue
			}
			if strings.Contains(path, deny) {
				return true, "path " + path + " matches deny-list entry " + deny
			}
		}
	}
	if reason := scanForHardcodedSecret(diffContent); reason != "" {
		return true, reason
	}
	return false, ""
}

// DefaultSecurityDenyListPaths is used when a project has not configured its
// own deny list.
var DefaultSecurityDenyListPaths = []string{".github/workflows/", "infra/"}

// scanForHardcodedSecret reuses MemoryService's secretPatterns regexp list
// (memory.go) — a single maintained pattern set for both the knowledge-graph
// redaction pass and this pre-commit guardrail, rather than two copies that
// can drift.
func scanForHardcodedSecret(diffContent string) string {
	for _, re := range secretPatterns {
		if re.MatchString(diffContent) {
			return "diff appears to match a hardcoded-secret pattern (" + re.String() + ")"
		}
	}
	return ""
}
