package tool

import (
	"encoding/json"
	"testing"
)

func TestResultJSONMarshaling(t *testing.T) {
	res := Result{
		Success:      true,
		Message:      "Replaced 1 occurrence",
		Output:       "success output",
		FilesChanged: []string{"server/main.go"},
		Diagnostics: []Diagnostic{
			{
				Severity: "error",
				File:     "server/main.go",
				Line:     10,
				Message:  "syntax error",
			},
		},
		Metadata: map[string]any{
			"hash_before": "abc",
			"hash_after":  "def",
		},
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Failed to marshal Result: %v", err)
	}

	var res2 Result
	if err := json.Unmarshal(data, &res2); err != nil {
		t.Fatalf("Failed to unmarshal Result: %v", err)
	}

	if !res2.Success {
		t.Errorf("Expected Success to be true")
	}
	if res2.Message != "Replaced 1 occurrence" {
		t.Errorf("Expected Message to be 'Replaced 1 occurrence'")
	}
	if len(res2.FilesChanged) != 1 || res2.FilesChanged[0] != "server/main.go" {
		t.Errorf("FilesChanged mismatch")
	}
	if len(res2.Diagnostics) != 1 || res2.Diagnostics[0].Message != "syntax error" {
		t.Errorf("Diagnostics mismatch")
	}
	if res2.Metadata["hash_before"] != "abc" {
		t.Errorf("Metadata mismatch")
	}
}
