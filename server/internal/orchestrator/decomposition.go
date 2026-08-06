package orchestrator

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// finalizeTaskAttempt closes out the TaskAttempt created at the start of
// run() for this task's execution (task-subtask-decomposition Phase 1/3).
// Best-effort: never fails the underlying workflow. exitStatus is 0 for a
// successful job, 1 for anything else — matching the coarse process-style
// exit-status semantics TaskAttempt.ExitStatus documents.
func (o *Orchestrator) finalizeTaskAttempt(ctx context.Context, attempt *models.TaskAttempt, job *models.WorkflowJob, exitStatus int) {
	if o.taskAttempts == nil || attempt == nil {
		return
	}
	input := models.FinalizeTaskAttemptInput{ExitStatus: &exitStatus}
	if job != nil {
		tokens := int(job.TotalTokensUsed)
		cost := job.TotalCostUSD
		input.TokensIn = &tokens
		input.CostUSD = &cost
	}
	if _, err := o.taskAttempts.Finalize(ctx, attempt.ID, input); err != nil {
		o.log(ctx, attempt.TaskID, nil, "warn", fmt.Sprintf("failed to finalize task attempt %s: %v", attempt.ID, err))
	}
}

// onChildTaskTerminal is invoked from updateTaskStatus (tracker.go) whenever
// a child task of a decomposition-managed parent reaches a terminal status.
// success=true (merged/done) advances to the next sibling by SequenceIndex,
// or runs Reduce once every child has succeeded; success=false blocks the
// parent, recording the failing child (specs.md Failure Scenario: "A child
// task fails").
func (o *Orchestrator) onChildTaskTerminal(ctx context.Context, child *models.Task, success bool) {
	if child == nil || child.ParentTaskID == nil {
		return
	}
	parent, err := o.tasks.GetByID(ctx, *child.ParentTaskID)
	if err != nil || parent.DecompositionMode == nil {
		// Not a decomposition-managed parent (e.g. a manually created
		// sub-task via the pre-existing CreateSubTask path) — leave it alone.
		return
	}

	if !success {
		childID := child.ID
		if _, err := o.tasks.Update(ctx, parent.ID, models.UpdateTaskInput{BlockedChildID: &childID}); err != nil {
			o.log(ctx, parent.ID, nil, "error", fmt.Sprintf("failed to record blocked_child_id: %v", err))
		}
		if _, err := o.updateTaskStatus(ctx, parent.ID, models.TaskStatusBlocked); err != nil {
			o.log(ctx, parent.ID, nil, "error", fmt.Sprintf("failed to transition parent to blocked: %v", err))
		}
		o.log(ctx, parent.ID, nil, "error", fmt.Sprintf(
			"metric task.decomposition.child_failure_rate parent_task_id=%s child_task_id=%s sequence_index=%v",
			parent.ID, child.ID, child.SequenceIndex))
		return
	}

	o.dispatchNextChildOrReduce(ctx, parent.ID)
}

// DispatchDecomposedParent starts sequential child execution for a parent
// whose split was just approved (or auto-proceeded). It is the exported
// entry point handler.ApproveSplit calls after TaskService.ApproveSplit has
// created the child Task rows.
func (o *Orchestrator) DispatchDecomposedParent(ctx context.Context, parentID string) error {
	o.dispatchNextChildOrReduce(ctx, parentID)
	return nil
}

// dispatchNextChildOrReduce finds the parent's children in SequenceIndex
// order and either enqueues the next unstarted/blocked-and-retried one, or —
// once every child has succeeded — runs Reduce and marks the parent
// complete. v1 dispatch is strictly sequential regardless of DependsOn
// (proposal.md Non-goals).
func (o *Orchestrator) dispatchNextChildOrReduce(ctx context.Context, parentID string) {
	children, err := o.tasks.ListSubTasks(ctx, parentID)
	if err != nil {
		o.log(ctx, parentID, nil, "error", fmt.Sprintf("failed to list children for dispatch: %v", err))
		return
	}
	ordered := steps.SortBySequenceIndex(children)

	allSucceeded := true
	for _, c := range ordered {
		if c.Status != models.TaskStatusMerged {
			allSucceeded = false
		}
		if c.Status == models.TaskStatusMerged {
			continue
		}
		if c.Status == models.TaskStatusFailed {
			// A prior failure should already have blocked the parent via
			// onChildTaskTerminal; this is a defensive stop, not the normal path.
			return
		}
		// First not-yet-succeeded child in sequence order: dispatch it if it
		// hasn't started yet. If it's already running/queued, do nothing —
		// this function is only ever reached after a sibling just finished.
		if c.Status == models.TaskStatusTodo || c.Status == "" {
			if _, err := o.Execute(ctx, c.ID); err != nil {
				o.log(ctx, parentID, nil, "error", fmt.Sprintf("failed to dispatch child %s (sequence_index=%v): %v", c.ID, c.SequenceIndex, err))
			} else {
				o.log(ctx, parentID, nil, "info", fmt.Sprintf("dispatched child %s (sequence_index=%v)", c.ID, c.SequenceIndex))
			}
		}
		return
	}

	if allSucceeded && len(ordered) > 0 {
		o.completeDecomposedParent(ctx, parentID, ordered)
	}
}

// RetryBlockedParent resumes a TaskStatusBlocked decomposed parent: it
// re-dispatches exactly the failed child recorded in BlockedChildID (a new
// TaskAttempt is created for that child's next run), leaving already-
// succeeded siblings untouched and children after it unstarted (specs.md
// Failure Scenario: "A child task fails" — resume behavior).
func (o *Orchestrator) RetryBlockedParent(ctx context.Context, parentID string) (*models.Task, error) {
	parent, err := o.tasks.GetByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if parent.Status != models.TaskStatusBlocked {
		return nil, fmt.Errorf("task is not blocked (current status: %s)", parent.Status)
	}
	if parent.BlockedChildID == nil {
		return nil, fmt.Errorf("blocked parent has no recorded failing child")
	}
	failedChildID := *parent.BlockedChildID

	if _, err := o.updateTaskStatus(ctx, parentID, models.TaskStatusCoding); err != nil {
		return nil, fmt.Errorf("failed to resume parent from blocked: %w", err)
	}
	if _, err := o.tasks.Update(ctx, parentID, models.UpdateTaskInput{ClearBlockedChild: true}); err != nil {
		o.log(ctx, parentID, nil, "warn", fmt.Sprintf("failed to clear blocked_child_id: %v", err))
	}

	if _, err := o.RetryFromLastStep(ctx, failedChildID); err != nil {
		return nil, fmt.Errorf("failed to retry blocked child %s: %w", failedChildID, err)
	}
	return o.tasks.GetByID(ctx, parentID)
}

// completeDecomposedParent runs the deterministic Reduce aggregation
// (Phase 4) and transitions the parent to TaskStatusMerged once every child
// has succeeded.
func (o *Orchestrator) completeDecomposedParent(ctx context.Context, parentID string, children []models.Task) {
	parent, err := o.tasks.GetByID(ctx, parentID)
	if err != nil {
		o.log(ctx, parentID, nil, "error", fmt.Sprintf("failed to load parent for reduce: %v", err))
		return
	}

	var attempts []models.TaskAttempt
	if o.taskAttempts != nil {
		ids := make([]string, len(children))
		for i, c := range children {
			ids[i] = c.ID
		}
		if a, aErr := o.taskAttempts.ListByTaskIDs(ctx, ids); aErr == nil {
			attempts = a
		}
	}

	summary := steps.Reduce(parent, children, attempts)
	if raw, mErr := steps.MarshalAnalysisPatch(summary, parent.Analysis); mErr == nil {
		if _, uErr := o.tasks.Update(ctx, parentID, models.UpdateTaskInput{Analysis: raw}); uErr != nil {
			o.log(ctx, parentID, nil, "warn", fmt.Sprintf("failed to persist decomposed_summary: %v", uErr))
		}
	}

	o.log(ctx, parentID, nil, "info", fmt.Sprintf(
		"metric task.decomposition.child_count value=%d parent_task_id=%s", len(children), parentID))
	o.log(ctx, parentID, nil, "info", fmt.Sprintf(
		"metric task.tokens.after_split value=%d parent_task_id=%s", summary.TokensAfterSplit, parentID))
	o.log(ctx, parentID, nil, "info", fmt.Sprintf(
		"metric task.duration.actual value=%d parent_task_id=%s", summary.DurationSeconds, parentID))
	o.log(ctx, parentID, nil, "info", fmt.Sprintf(
		"metric task.cost.saved value=%.4f parent_task_id=%s", summary.CostSavedUSD, parentID))

	if _, err := o.updateTaskStatus(ctx, parentID, models.TaskStatusMerged); err != nil {
		o.log(ctx, parentID, nil, "error", fmt.Sprintf("failed to mark decomposed parent complete: %v", err))
	}

	// Every child shares the parent's on-disk workspace (models.
	// WorkspaceOwnerID); a child's own terminal state intentionally skips
	// disk cleanup (see CleanupWorkspaceAfterFinalState) so the next
	// sibling still has the clone to build on. Now that the whole family
	// is done, clean up the shared workspace once, keyed by the parent's
	// own ID (which is also the workspace owner ID, since a parent has no
	// ParentTaskID of its own).
	o.cleanupWorkspaceAfterFinalState(context.WithoutCancel(ctx), parentID)
}
