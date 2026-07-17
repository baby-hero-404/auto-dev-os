# Specs: Status-Driven Task Detail UI

## Added Requirements

### REQ-001: Status-Driven Component Visibility
> ✅ Status: Implemented

**Scenario: Task is in TODO state**
- WHEN a task has `status === "todo"`
- THEN the Hero section displays the Task Description
- AND execution components (Logs, Timeline, Code Review) are hidden.

**Scenario: Task is in SPEC REVIEW state**
- WHEN a task has `status === "spec_review"`
- THEN the Hero section displays the `Definition-of-Ready Gate` (rendered markdown of the spec)
- AND execution components are hidden.

**Scenario: Task is in EXECUTION state**
- WHEN a task is in `coding`, `testing`, or `fixing`
- THEN the Hero section displays the Live Execution Console (LogConsole + Checklists).

**Scenario: Task is in FAILED state**
- WHEN a task has `status === "failed"`
- THEN the Hero section highlights the error
- AND displays the latest execution logs within the hero card
- AND displays boundary resolution controls if applicable.

**Scenario: Task is MERGED**
- WHEN a task has `status === "merged"`
- THEN the Hero section displays a success summary
- AND active execution panels are hidden.

### REQ-002: Description Proximity & Collapsibility
> ✅ Status: Implemented

**Scenario:**
- WHEN a user views any task
- THEN the task description is available immediately under the Task Title in `TaskTitleBlock`.
- AND it is truncated to 2 lines by default, with a "Show more" toggle to expand it.

### REQ-003: Persistent Access to Context
> ✅ Status: Implemented

**Scenario:**
- WHEN a task enters `merged` state and the Hero section hides execution components
- THEN the user can still access the full execution logs and spec via the `SupportingAccordion` at the bottom of the page.
- AND the user can use `SupportingAccordion` to edit the description if necessary.

## Modified Requirements

### REQ-M01: Revert Breadcrumb Navigation
> ✅ Status: Implemented

**Scenario:**
- WHEN viewing a task
- THEN the top-left navigation shows `← Back to Project` instead of a broken `Projects / Task Name` path, matching the old functional design.
