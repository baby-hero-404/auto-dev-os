package steps

import (
	"encoding/json"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// LoadFrozenContext attempts to load the FrozenContext from the plan step output.
// Falls back to building a FrozenContext from live TaskAnalysis if not available
// (backward compatibility with older workflows).
func LoadFrozenContext(stepCtx workflow.StepContext, analysis *models.TaskAnalysis) *models.FrozenContext {
	// Try loading from plan step output
	if planOut, ok := stepCtx.Inputs[workflow.StepPlan]; ok {
		if frozenJSON, ok := planOut["frozen_context"].(string); ok && frozenJSON != "" {
			var frozen models.FrozenContext
			if err := json.Unmarshal([]byte(frozenJSON), &frozen); err == nil {
				return &frozen
			}
		}
	}

	// Fallback: build from live TaskAnalysis (backward compat)
	if analysis == nil {
		return nil
	}
	return &models.FrozenContext{
		SpecHash:            analysis.SpecHash,
		ProposalMD:          analysis.ProposalMD,
		SpecsMD:             analysis.SpecsMD,
		DesignMD:            analysis.DesignMD,
		TasksMD:             analysis.TasksMD,
		ExecutionUnits:      analysis.ExecutionUnits,
		ExecutionBoundaries: analysis.ExecutionBoundaries,
		AffectedFiles:       analysis.AffectedFiles,
		AcceptanceCriteria:  analysis.AcceptanceCriteria,
		ExecutionPhases:     analysis.ExecutionPhases,
		Risks:               analysis.Risks,
		RiskDomains:         analysis.RiskDomains,
	}
}
