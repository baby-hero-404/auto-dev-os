package prompts

import (
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

func TestLoadStepPrompt_CLISteps(t *testing.T) {
	a := NewPromptAssemblerWithRules(nil, nil, paths.NewOSPromptPaths("."), paths.NewOSFileSystem(), &MockContextEngine{})

	cases := map[string]string{
		"cli_analyze":   ".autocode/analysis.md",
		"cli_spec":      "docs/openspecs/<task-slug>/",
		"cli_implement": "tasks.md",
	}
	for stepID, want := range cases {
		content, err := a.LoadStepPrompt(stepID)
		if err != nil {
			t.Fatalf("LoadStepPrompt(%q): unexpected error: %v", stepID, err)
		}
		if !strings.Contains(content, want) {
			t.Errorf("LoadStepPrompt(%q): expected content to contain %q, got:\n%s", stepID, want, content)
		}
	}
}
