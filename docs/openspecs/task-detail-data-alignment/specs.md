# Specs: Task Detail Data Alignment

## Modified Requirements

### REQ-M01: Dynamic Hero Cards Data Binding
> ⚠️ Status: In Progress

**Scenario:**
- WHEN a task is in the `spec_review` state
- THEN the Hero Card must render the specification content from `analysisData.proposal_md`, `specs_md`, or fallback gracefully if empty
- AND the execution log view must map directly to the `logs` array, handling the auto-scrolling or latest items accurately without visual overflow.

### REQ-M02: Accurate Workflow Stepper
> ⚠️ Status: In Progress

**Scenario:**
- WHEN the user views the right sidebar timeline
- THEN the stepper must reflect the actual `workflowSteps` array computed by `useTaskDetail` (which handles easy/medium/hard complexities) instead of hardcoded phases.
- AND the step status (completed, running, pending, failed) must be derived from the `latest` map provided by the context.

### REQ-M03: Implementation Checklist Accuracy
> ⚠️ Status: In Progress

**Scenario:**
- WHEN a task is in the `coding` or `testing` phase
- THEN the subtasks list must reflect the exact `implementationItems` array derived from execution units.
- AND the completion percentage must exactly match the ratio of `done` items to total items.
