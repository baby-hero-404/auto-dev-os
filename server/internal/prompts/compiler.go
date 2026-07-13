package prompts

import (
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// PromptCompiler defines the interface for rendering ExecutionIR into provider-specific prompts.
type PromptCompiler interface {
	Compile(ir models.ExecutionIR, physicalTargets []string) ([]llm.Message, error)
}

// DefaultPromptCompiler provides a basic rendering implementation.
type DefaultPromptCompiler struct {
	Provider string
}

func NewDefaultPromptCompiler(provider string) *DefaultPromptCompiler {
	return &DefaultPromptCompiler{Provider: provider}
}

func (c *DefaultPromptCompiler) Compile(ir models.ExecutionIR, physicalTargets []string) ([]llm.Message, error) {
	// Reject invalid IR before compiling (REQ-001): structured, field-level errors.
	if err := models.ValidateExecutionIR(ir); err != nil {
		return nil, fmt.Errorf("invalid Execution IR: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("=== Execution Contract ===\n")
	sb.WriteString(fmt.Sprintf("Node ID: %s\n", ir.NodeID))
	sb.WriteString(fmt.Sprintf("Capability: %s\n", ir.Intent.Capability))
	sb.WriteString(fmt.Sprintf("Operation: %s\n\n", ir.Intent.Operation))

	if len(physicalTargets) > 0 {
		sb.WriteString("=== Physical Targets (Write Scope) ===\n")
		for _, target := range physicalTargets {
			sb.WriteString(fmt.Sprintf("- %s\n", target))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== Constraints ===\n")
	for _, c := range ir.Constraints {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	sb.WriteString("\n")

	sb.WriteString("=== Acceptance Criteria ===\n")
	for _, a := range ir.Acceptance {
		sb.WriteString(fmt.Sprintf("- %s\n", a))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("=== Budgets ===\nDiscovery: %d | Implementation: %d | Validation: %d\n",
		ir.Budget.Discovery, ir.Budget.Implementation, ir.Budget.Validation))

	// For specific providers, we might format this differently (e.g., using <anthropic-tags> for Claude).
	if c.Provider == "anthropic" {
		wrapped := fmt.Sprintf("<execution_contract>\n%s</execution_contract>", sb.String())
		return []llm.Message{{Role: "user", Content: wrapped}}, nil
	}

	return []llm.Message{{Role: "user", Content: sb.String()}}, nil
}
