package service

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func actionIDs(actions []models.AvailableAction) map[string]bool {
	ids := make(map[string]bool, len(actions))
	for _, a := range actions {
		ids[a.ID] = true
	}
	return ids
}

func TestComputeAvailableActions_Coding(t *testing.T) {
	got := actionIDs(computeAvailableActions(&models.Task{Status: models.TaskStatusCoding}))
	want := map[string]bool{"pause": true, "cancel": true}
	if len(got) != len(want) || !got["pause"] || !got["cancel"] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeAvailableActions_SpecReview(t *testing.T) {
	got := actionIDs(computeAvailableActions(&models.Task{Status: models.TaskStatusSpecReview}))
	for _, id := range []string{"approve_spec", "request_changes", "cancel"} {
		if !got[id] {
			t.Fatalf("expected %q in spec_review actions, got %v", id, got)
		}
	}
}

func TestComputeAvailableActions_CodingWithProposedSplitUnaffected(t *testing.T) {
	task := &models.Task{Status: models.TaskStatusCoding, ComplexityScore: []byte(`{"score":9}`)}
	got := actionIDs(computeAvailableActions(task))
	if got["approve_split"] || got["reject_split"] {
		t.Fatalf("proposed_split must never introduce approve_split/reject_split, got %v", got)
	}
	if len(got) != 2 || !got["pause"] || !got["cancel"] {
		t.Fatalf("expected normal coding action set, got %v", got)
	}
}

func TestComputeAvailableActions_PrReadyAndHumanReview(t *testing.T) {
	for _, status := range []string{models.TaskStatusPrReady, models.TaskStatusHumanReview} {
		got := actionIDs(computeAvailableActions(&models.Task{Status: status}))
		if len(got) != 1 || !got["cancel"] {
			t.Fatalf("status %s: expected only cancel, got %v", status, got)
		}
		if got["approve_merge"] || got["reject_merge"] {
			t.Fatalf("status %s: must never expose approve_merge/reject_merge", status)
		}
	}
}

func TestComputeAvailableActions_Merged(t *testing.T) {
	got := computeAvailableActions(&models.Task{Status: models.TaskStatusMerged})
	if len(got) != 0 {
		t.Fatalf("expected empty actions for merged, got %v", got)
	}
}

func TestComputeAvailableActions_Blocked(t *testing.T) {
	got := actionIDs(computeAvailableActions(&models.Task{Status: models.TaskStatusBlocked}))
	if len(got) != 2 || !got["retry_blocked"] || !got["cancel"] {
		t.Fatalf("expected retry_blocked+cancel, got %v", got)
	}
}

// TestComputeAvailableActions_NoApprovalLeak asserts, for every TaskStatus
// except spec_review, that no approval-style action ID is ever returned.
// This is the invariant computeAvailableActions exists to enforce (specs.md
// invariant #2): spec_review is the only human-approval-gated status.
func TestComputeAvailableActions_NoApprovalLeak(t *testing.T) {
	approvalIDs := map[string]bool{
		"approve_spec": true, "request_changes": true,
		"approve_split": true, "reject_split": true,
		"approve_merge": true, "reject_merge": true,
	}
	allowed := map[string]bool{"pause": true, "cancel": true, "retry": true, "retry_blocked": true, "delete": true, "execute": true}

	statuses := []string{
		models.TaskStatusTodo, models.TaskStatusContextLoading, models.TaskStatusAnalyzing,
		models.TaskStatusCoding, models.TaskStatusReviewing, models.TaskStatusFixing,
		models.TaskStatusTesting, models.TaskStatusPrReady, models.TaskStatusHumanReview,
		models.TaskStatusMerged, models.TaskStatusFailed, models.TaskStatusBlocked,
	}

	for _, status := range statuses {
		for _, action := range computeAvailableActions(&models.Task{Status: status}) {
			if approvalIDs[action.ID] {
				t.Fatalf("status %s leaked approval-style action %q", status, action.ID)
			}
			if !allowed[action.ID] {
				t.Fatalf("status %s returned unexpected action %q", status, action.ID)
			}
		}
	}
}
