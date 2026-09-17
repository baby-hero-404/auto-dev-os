package prompts

import (
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// stepRequiresEditCaps reports whether stepID's instruction expects the CLI
// agent to modify files. Analyze, spec-authoring and review steps read; the
// implement steps write.
func stepRequiresEditCaps(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCLIImplement)
}

// EffectiveRoleForStep returns the role a step renders its persona with.
//
// This used to live in internal/tool, where it was the single source of truth
// across three consumers: tool advertisement, tool execution boundaries, and
// persona selection — the whole point being that advertise/enforce/persona
// must not diverge (task 8291a25e: fix under a planner/reviewer persona had
// zero edit tools while its instruction demanded edits). Two of those three
// consumers are gone with the server-side tool loop: the CLI harness brings
// its own tools and enforces its own boundaries, so only persona selection
// remains and the capability lookup it used to do has nothing left to check.
// What survives is the remap itself — an implement step must never render a
// read-only persona, because the persona is now the only role signal the CLI
// agent receives.
func EffectiveRoleForStep(stepID, agentRole string, task *models.Task) string {
	if !stepRequiresEditCaps(stepID) {
		return agentRole
	}
	if agentRole == models.AgentRoleBackend || agentRole == models.AgentRoleFrontend {
		return agentRole // already a coder role — keep it
	}
	return CoderRoleForTask(task)
}

// CoderRoleForTask returns "frontend" when the task's analysis names a
// frontend-shaped primary category, else "backend". The remap target for
// EffectiveRoleForStep.
func CoderRoleForTask(task *models.Task) string {
	if task == nil || len(task.Analysis) == 0 {
		return models.AgentRoleBackend
	}
	var analysis models.TaskAnalysis
	if err := json.Unmarshal(task.Analysis, &analysis); err == nil {
		switch strings.ToLower(analysis.PrimaryCategory) {
		case "frontend", "ui", "ux":
			return models.AgentRoleFrontend
		}
	}
	return models.AgentRoleBackend
}
