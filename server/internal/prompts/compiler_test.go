package prompts

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestDefaultPromptCompiler_Compile(t *testing.T) {
	ir := models.ExecutionIR{
		SchemaVersion: models.CurrentExecutionIRSchemaVersion,
		NodeID:        "node_123",
		Intent: models.Intent{
			Capability: "UserRepository",
			Operation:  "create",
		},
		Constraints: []string{"Must use interface", "Must include tests"},
		Acceptance:  []string{"Tests pass", "Implements methods"},
		Budget: models.PhaseBudgets{
			Discovery:      5,
			Implementation: 10,
			Validation:     5,
		},
	}

	targets := []string{"internal/repository/user.go"}

	tests := []struct {
		provider string
		golden   string
	}{
		{"default", "testdata/compiler_default.golden"},
		{"anthropic", "testdata/compiler_anthropic.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			compiler := NewDefaultPromptCompiler(tt.provider)
			msgs, err := compiler.Compile(ir, targets)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(msgs) != 1 {
				t.Fatalf("expected 1 message, got %d", len(msgs))
			}

			actualBytes, _ := json.MarshalIndent(msgs, "", "  ")

			// Check against golden file
			if *updateGolden {
				os.MkdirAll("testdata", 0755)
				os.WriteFile(tt.golden, actualBytes, 0644)
			}

			expectedBytes, err := os.ReadFile(tt.golden)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file %s does not exist. run tests with -update flag.", tt.golden)
				}
				t.Fatalf("failed to read golden file: %v", err)
			}

			if string(expectedBytes) != string(actualBytes) {
				t.Errorf("compiled prompt does not match golden file.\nExpected:\n%s\n\nActual:\n%s", string(expectedBytes), string(actualBytes))
			}
		})
	}
}

func TestDefaultPromptCompiler_Compile_InvalidIR(t *testing.T) {
	compiler := NewDefaultPromptCompiler("default")

	// Missing schema_version, node_id, and an invalid operation enum.
	invalid := models.ExecutionIR{
		Intent:      models.Intent{Capability: "UserRepository", Operation: "invent"},
		Constraints: []string{},
		Acceptance:  []string{},
	}

	_, err := compiler.Compile(invalid, nil)
	if err == nil {
		t.Fatal("expected error for invalid Execution IR, got nil")
	}
	for _, want := range []string{"schema_version", "node_id", "intent.operation"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to mention %q, got: %v", want, err)
		}
	}
}
