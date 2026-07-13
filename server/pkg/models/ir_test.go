package models

import (
	"strings"
	"testing"
)

func validIR() ExecutionIR {
	return ExecutionIR{
		SchemaVersion: CurrentExecutionIRSchemaVersion,
		NodeID:        "node_1",
		Intent:        Intent{Capability: "UserRepository", Operation: "create"},
		Constraints:   []string{"must use interface"},
		Acceptance:    []string{"tests pass"},
		Budget:        PhaseBudgets{Discovery: 3, Implementation: 6, Validation: 2},
	}
}

func TestParseExecutionIR_Valid(t *testing.T) {
	raw := `{
		"schema_version": "1.0",
		"node_id": "node_1",
		"intent": {"capability": "UserRepository", "operation": "create"},
		"constraints": ["must use interface"],
		"acceptance": ["tests pass"],
		"budget": {"discovery": 3, "implementation": 6, "validation": 2}
	}`

	ir, err := ParseExecutionIR([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir.NodeID != "node_1" || ir.Intent.Operation != "create" {
		t.Errorf("unexpected decoded IR: %+v", ir)
	}
}

func TestParseExecutionIR_RejectsUnknownFields(t *testing.T) {
	raw := `{
		"schema_version": "1.0",
		"node_id": "node_1",
		"intent": {"capability": "UserRepository", "operation": "create"},
		"constraints": [],
		"acceptance": [],
		"budget": {"discovery": 1, "implementation": 1, "validation": 1},
		"unexpected_field": "should fail"
	}`

	if _, err := ParseExecutionIR([]byte(raw)); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestParseExecutionIR_RejectsWrongSchemaVersion(t *testing.T) {
	raw := `{
		"schema_version": "2.0",
		"node_id": "node_1",
		"intent": {"capability": "UserRepository", "operation": "create"},
		"constraints": [],
		"acceptance": [],
		"budget": {"discovery": 1, "implementation": 1, "validation": 1}
	}`

	_, err := ParseExecutionIR([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected schema_version error, got: %v", err)
	}
}

func TestValidateExecutionIR_ValidPasses(t *testing.T) {
	if err := ValidateExecutionIR(validIR()); err != nil {
		t.Fatalf("expected valid IR to pass, got: %v", err)
	}
}

func TestValidateExecutionIR_MissingRequiredFields(t *testing.T) {
	ir := ExecutionIR{}
	err := ValidateExecutionIR(ir)
	if err == nil {
		t.Fatal("expected error for empty IR, got nil")
	}
	for _, want := range []string{"schema_version", "node_id", "intent.capability", "intent.operation", "constraints", "acceptance"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to mention %q, got: %v", want, err)
		}
	}
}

func TestValidateExecutionIR_InvalidOperationEnum(t *testing.T) {
	ir := validIR()
	ir.Intent.Operation = "destroy"
	err := ValidateExecutionIR(ir)
	if err == nil || !strings.Contains(err.Error(), "intent.operation") {
		t.Fatalf("expected intent.operation error, got: %v", err)
	}
}

func TestValidateExecutionIR_NegativeBudget(t *testing.T) {
	ir := validIR()
	ir.Budget.Implementation = -1
	err := ValidateExecutionIR(ir)
	if err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("expected budget error, got: %v", err)
	}
}

func TestExecutionIRSchema_MatchesGoContract(t *testing.T) {
	schema := ExecutionIRSchema()
	if schema["additionalProperties"] != false {
		t.Error("expected additionalProperties: false in embedded schema")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok || props["schema_version"] == nil {
		t.Error("expected schema_version in embedded schema properties")
	}
}
