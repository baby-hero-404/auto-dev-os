package tool

import (
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// StepRequiresEditCaps reports whether stepID's instruction expects the model to modify files
// (fix + all coding steps; review/analyze/plan stay read-only).
func StepRequiresEditCaps(stepID string) bool {
	return stepID == workflow.StepFix ||
		strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend)
}

// EffectiveRoleForStep returns the capability role a step's LLM call should advertise, execute,
// AND render its persona with. Single source of truth used by tool advertisement
// (CapabilityManager.ToolsForRole), tool execution (NewBoundaryCheckedToolExecutor), and prompt
// persona selection (PromptAssembler.loadBaseRolePrompts) — advertise/enforce/persona diverging
// is exactly the bug this function exists to prevent (task 8291a25e: fix under planner/reviewer
// had zero edit tools while its instruction demanded edits).
//
// If the step expects edits but the assigned agent's role lacks CapEdit/CapCreate, remap to the
// task's coder role.
func EffectiveRoleForStep(stepID, agentRole string, task *models.Task) string {
	if !StepRequiresEditCaps(stepID) {
		return agentRole
	}
	if AllowedForRole(agentRole, []Capability{CapEdit}) &&
		AllowedForRole(agentRole, []Capability{CapCreate}) {
		return agentRole // already a coder role — keep it
	}
	return CoderRoleForTask(task)
}

// CoderRoleForTask returns "frontend" when the task's analysis names a frontend-shaped primary
// category, else "backend". Used as the remap target by EffectiveRoleForStep.
func CoderRoleForTask(task *models.Task) string {
	if task == nil || len(task.Analysis) == 0 {
		return "backend"
	}
	var analysis models.TaskAnalysis
	if err := json.Unmarshal(task.Analysis, &analysis); err == nil {
		switch strings.ToLower(analysis.PrimaryCategory) {
		case "frontend", "ui", "ux":
			return "frontend"
		}
	}
	return "backend"
}
