package policy

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func boolPtr(b bool) *bool { return &b }

func TestRequiresSpecReview_DefaultIsAuto(t *testing.T) {
	if RequiresSpecReview(&models.Task{}, &models.Project{}) {
		t.Error("a task with nothing configured must run straight through to the MR")
	}
}

func TestRequiresSpecReview_ProjectDefaultApplies(t *testing.T) {
	if !RequiresSpecReview(&models.Task{}, &models.Project{RequireSpecReview: true}) {
		t.Error("expected the project default to gate a task that does not override it")
	}
}

// An explicit false on the task must beat a project default of true —
// otherwise the per-task control is only half a control.
func TestRequiresSpecReview_TaskOverrideBeatsProject(t *testing.T) {
	if RequiresSpecReview(&models.Task{RequireSpecReview: boolPtr(false)}, &models.Project{RequireSpecReview: true}) {
		t.Error("an explicit task-level false must override a project default of true")
	}
	if !RequiresSpecReview(&models.Task{RequireSpecReview: boolPtr(true)}, &models.Project{RequireSpecReview: false}) {
		t.Error("an explicit task-level true must override a project default of false")
	}
}

func TestResolveSpecGate_AutoByDefault(t *testing.T) {
	specStatus, taskStatus := ResolveSpecGate(&models.Task{}, &models.Project{}, false, 0)
	if specStatus != models.TaskSpecStatusAutoApproved || taskStatus != models.TaskStatusCoding {
		t.Errorf("expected auto-approve straight to coding, got (%s, %s)", specStatus, taskStatus)
	}
}

func TestResolveSpecGate_FlagPauses(t *testing.T) {
	specStatus, taskStatus := ResolveSpecGate(&models.Task{}, &models.Project{RequireSpecReview: true}, false, 0)
	if specStatus != models.TaskSpecStatusPendingReview || taskStatus != models.TaskStatusSpecReview {
		t.Errorf("expected a pause at spec_review, got (%s, %s)", specStatus, taskStatus)
	}
}

// Clarifications gate regardless of the flag: proceeding would silently
// answer the agent's own questions by assumption.
func TestResolveSpecGate_ClarificationsGateEvenInAutoMode(t *testing.T) {
	specStatus, taskStatus := ResolveSpecGate(&models.Task{}, &models.Project{}, true, 0)
	if specStatus != models.TaskSpecStatusClarificationRequired || taskStatus != models.TaskStatusSpecReview {
		t.Errorf("expected clarification gating in auto mode, got (%s, %s)", specStatus, taskStatus)
	}
}

// ...but they must not deadlock a task whose author never answers.
func TestResolveSpecGate_ClarificationBypassAfterRoundLimit(t *testing.T) {
	specStatus, taskStatus := ResolveSpecGate(&models.Task{}, &models.Project{}, true, MaxClarificationRounds)
	if specStatus != models.TaskSpecStatusReadyWithWarnings || taskStatus != models.TaskStatusCoding {
		t.Errorf("expected the round limit to release the task with warnings, got (%s, %s)", specStatus, taskStatus)
	}
}

// A hotfix bypasses the clarification gate but still honours an explicit
// request for spec review — the two are independent controls.
func TestResolveSpecGate_HotfixBypassStillRespectsFlag(t *testing.T) {
	task := &models.Task{Labels: []string{"hotfix"}}
	specStatus, taskStatus := ResolveSpecGate(task, &models.Project{}, true, 0)
	if specStatus != models.TaskSpecStatusReadyWithWarnings || taskStatus != models.TaskStatusCoding {
		t.Errorf("expected hotfix to bypass the clarification gate, got (%s, %s)", specStatus, taskStatus)
	}
}

func TestIsDefinitionOfReadyBypassed(t *testing.T) {
	tests := []struct {
		name       string
		labels     []string
		priorRound int
		want       bool
	}{
		{name: "no labels, no rounds", labels: nil, priorRound: 0, want: false},
		{name: "one round is within limit", labels: nil, priorRound: 1, want: false},
		{name: "round limit reached bypasses", labels: nil, priorRound: 2, want: true},
		{name: "round limit exceeded bypasses", labels: nil, priorRound: 5, want: true},
		{name: "hotfix label bypasses regardless of rounds", labels: []string{"hotfix"}, priorRound: 0, want: true},
		{name: "hotfix label case-insensitive", labels: []string{"Hotfix"}, priorRound: 0, want: true},
		{name: "unrelated label does not bypass", labels: []string{"bug"}, priorRound: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDefinitionOfReadyBypassed(tt.labels, tt.priorRound)
			if got != tt.want {
				t.Errorf("IsDefinitionOfReadyBypassed(%v, %d) = %v, want %v", tt.labels, tt.priorRound, got, tt.want)
			}
		})
	}
}
