# Implementation Plan: Execution Unit DAG & Dynamic Scheduler

## 1. Objective
Refactor the orchestration pipeline to replace the legacy `ExecutionPhases` string array with a structured `Execution Unit` model. Implement a dynamic scheduler in the `Plan` step to perform Granularity and Phase Cost validation before distributing work to specialized coding agents.

## 2. Requirements
- Modify `models.TaskAnalysis` to use `ExecutionUnits` instead of `ExecutionPhases`.
- Update `Analyze` prompts to generate the new JSON metadata (`execution_profile` and `constraints`).
- Update `Plan` step logic to transition from keyword-heuristic mapping to explicit DAG scheduling.
- Introduce `Phase Cost` calculation to reject execution units that exceed complexity thresholds.

## 3. Implementation Steps

### Step 1: Update Domain Models
**File**: `server/pkg/models/task.go`
- Deprecate `ExecutionPhase` (or keep for backward compatibility).
- Introduce new structs: `ExecutionProfile`, `ExecutionConstraints`, and `ExecutionUnit`.
- Add `ExecutionUnits []ExecutionUnit` to `TaskAnalysis`.

### Step 2: Refactor Analyze Step (LLM Generation)
**File**: `server/internal/orchestrator/steps/analyze.go`
- Update the parser in `applyAnalyzePolicy` to parse the new `execution_units` JSON array.
- Support reading `dependencies`, `execution_profile.agent`, `execution_profile.skills`, and constraints.
- Maintain fallback compatibility for older specs.

**File**: `server/internal/prompts/steps/analyze.md`
- Provide strict guidelines on Granularity Control (Rule of Isolation, Context Limit Rule).
- Provide the JSON schema for `ExecutionUnit`.

### Step 3: Introduce Cost Engine & Scheduler Policy
**File**: `server/internal/orchestrator/policy/scheduler.go` (New)
- Define a function `CalculatePhaseCost(unit ExecutionUnit) int` based on the heuristic rules:
  - `Modify` = 1
  - `Create` = 2
  - `Config/Dependency` = 3
  - `Migration` = 5
  - `Test` = 1
- Implement a `ValidateDAG(units []ExecutionUnit)` function to check for:
  - Phase Cost > Threshold (e.g., 8).
  - Cyclic dependencies.
  - Exceeding max agent quota.

### Step 4: Refactor Plan Step (The Scheduler)
**File**: `server/internal/orchestrator/steps/plan.go` & `workflow/parser.go`
- Replace `classifyHeading` keyword guessing with direct reading of `unit.ExecutionProfile.Agent`.
- Run the new `policy.ValidateDAG()`. If it fails, return a `workflow.PauseError` to pause the job and request human intervention (or feed back into Analyze for an auto-split retry).
- Construct the final `workflow.Definition` explicitly honoring the `dependencies` array to build the DAG instead of simple sequential backend/frontend chains.

## 4. Rollout Strategy
1. Implement Data Models & Scheduler Policy logic.
2. Update the `Analyze` step and Prompts.
3. Hook the Policy validator into the `Plan` step.
4. Verify end-to-end functionality using tests simulating large task scopes.
