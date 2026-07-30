package engine

import "testing"

func TestDetectAwaitingInput_YesNoPrompt(t *testing.T) {
	if !detectAwaitingInput("Proceed with deletion? (y/n)") {
		t.Error("expected match for (y/n) prompt")
	}
}

func TestDetectAwaitingInput_DoYouWantTo(t *testing.T) {
	if !detectAwaitingInput("Do you want to overwrite the existing config?") {
		t.Error("expected match for 'Do you want to...?' prompt")
	}
}

func TestDetectAwaitingInput_NormalOutput(t *testing.T) {
	if detectAwaitingInput("Task completed successfully.") {
		t.Error("expected no match for normal completion message")
	}
}
