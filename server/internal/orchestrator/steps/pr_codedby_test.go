package steps

import (
	"encoding/json"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestCodedByReviewedBy_ExtractsFromMostRecentReviewCheckpoint(t *testing.T) {
	state, _ := json.Marshal(map[string]any{
		"output": map[string]any{
			"coded_by":    map[string]any{"engine": "api_native", "provider": "anthropic", "model": "claude-sonnet-5"},
			"reviewed_by": map[string]any{"provider": "openai", "model": "gpt-5"},
		},
	})
	checkpoints := []models.WorkflowCheckpoint{
		{Step: workflow.StepReview, State: state},
	}

	codedBy, reviewedBy := codedByReviewedBy(checkpoints)

	if codedBy != "api_native:anthropic/claude-sonnet-5" {
		t.Errorf("unexpected codedBy: %s", codedBy)
	}
	if reviewedBy != "openai/gpt-5" {
		t.Errorf("unexpected reviewedBy: %s", reviewedBy)
	}
}

func TestCodedByReviewedBy_EmptyWhenNoReviewCheckpoint(t *testing.T) {
	checkpoints := []models.WorkflowCheckpoint{
		{Step: workflow.StepCodeBackend, State: json.RawMessage(`{}`)},
	}

	codedBy, reviewedBy := codedByReviewedBy(checkpoints)

	if codedBy != "" || reviewedBy != "" {
		t.Errorf("expected empty results, got codedBy=%s reviewedBy=%s", codedBy, reviewedBy)
	}
}
