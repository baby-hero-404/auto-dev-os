package workflow

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestStatusForStep_CLIAnalyze(t *testing.T) {
	if got := StatusForStep(StepCLIAnalyze); got != models.TaskStatusAnalyzing {
		t.Errorf("expected TaskStatusAnalyzing, got %q", got)
	}
}

func TestStatusForStep_CLISpec(t *testing.T) {
	if got := StatusForStep(StepCLISpec); got != models.TaskStatusAnalyzing {
		t.Errorf("expected TaskStatusAnalyzing, got %q", got)
	}
}

func TestStatusForStep_CLIImplement(t *testing.T) {
	if got := StatusForStep(StepCLIImplement); got != models.TaskStatusCoding {
		t.Errorf("expected TaskStatusCoding, got %q", got)
	}
}

func TestStatusForStep_UnknownDefaultsToAnalyzing(t *testing.T) {
	if got := StatusForStep("some_unknown_step"); got != models.TaskStatusAnalyzing {
		t.Errorf("expected TaskStatusAnalyzing as the safe default, got %q", got)
	}
}
