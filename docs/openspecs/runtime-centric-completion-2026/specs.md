# Specs: Runtime-Centric Completion 2026

## Added Requirements

### REQ-001: Measurable Rollout Gate for State Machine
> ❌ Status: Not Started

**Scenario: shadow FSM violation rate is below threshold**
- WHEN the shadow state machine (flag off) has observed at least N completed tasks since the
  last gate check
- AND the `[TELEMETRY-VIOLATION]` warn-log rate over those tasks is below the configured
  threshold `execution.rollout_violation_threshold_pct` (default 1.0, i.e. <1% of LLM calls)
- THEN the rollout gate reports "pass" and `state_machine_enabled` may be flipped to `true` for a
  canary subset of tasks
- AND the gate's pass/fail decision and the measured rate are printed as a single structured
  `GateResult` JSON report, not just left implicit in raw warn logs
- AND the sample is drawn via `WorkflowRepo`/`TaskRepo` (file- or DB-backed, whichever mode is
  active), not a direct SQL query — task logs are not guaranteed to live in Postgres

**Scenario: shadow FSM violation rate is above threshold**
- WHEN the violation rate exceeds the threshold
- THEN the gate reports "fail" with the top violation types and their counts
- AND `state_machine_enabled` stays `false`

### REQ-002: Legacy Loop and Assembler Retirement (gated, follow-up execution)
> ❌ Status: Not Started

**Scenario: gate has passed for one full release cycle**
- WHEN REQ-001's gate has reported "pass" continuously for one full release cycle with
  `state_machine_enabled=true` on 100% of traffic
- THEN `RunToolLoop` (`llmrunner/toolloop.go`) and its exclusive callers are removed
- AND the legacy prompt assembler path (`internal/prompts/builder.go`/`assembler.go`) is removed
  or reduced to whatever the Prompt Compiler still delegates to
- AND no step can reach `runAgentic`/shadow-FSM code anymore — `runStateMachine` is the only path

### REQ-003: Review Includes Execution Unit Objectives
> ❌ Status: Not Started

**Scenario: review instruction is built for a task with frozen execution units**
- WHEN `ReviewStep.Execute` builds the review instruction for a task with a `FrozenContext`
- THEN the instruction includes the `Objective` of **every** frozen execution unit, keyed by unit
  ID (the review diff is the merged result of all units), in addition to the existing diff,
  Acceptance Criteria, Execution Boundaries and `TasksMD` blocks
- AND the reviewer's findings can be traced to whether the diff satisfies those stated
  objectives, not only whether it satisfies the acceptance-criteria checklist

**Scenario: no frozen objectives are available (legacy task)**
- WHEN no `FrozenContext` exists, or no frozen unit has a non-empty `Objective`
- THEN the review instruction is built exactly as today (diff + AC + boundaries), unchanged
- AND no error or warning is raised — the objectives block is additive, not required

### REQ-004: Execution Unit File-Count Enforcement at PLAN_READY
> ❌ Status: Not Started

**Scenario: resolved targets exceed the unit's own MaxFiles**
- WHEN Intent Resolver produces resolved targets for a node at `PLAN_READY`
- AND `len(resolvedTargets) > unit.Constraints.MaxFiles`
- THEN the node fails at `PLAN_READY` with a structured error naming the resolved file count and
  the declared `MaxFiles`
- AND the failure surfaces the same way Task 1.2's "unresolvable intent" failure does — hard
  failure when `state_machine_enabled` is on, warn-log-only when off (same flag semantics as the
  existing unresolvable-intent handling, `intent_resolver.go:133-138`) — so a flag-on task never
  enters `IMPLEMENTATION` over-scoped

**Scenario: resolved targets are within budget**
- WHEN `len(resolvedTargets) <= unit.Constraints.MaxFiles`
- THEN `PLAN_READY` proceeds unchanged

### REQ-005: Semantic Hash Independent of Prompt Wording
> ❌ Status: Not Started

**Scenario: prompt wording changes but execution state does not**
- WHEN two `ExecutionSnapshot`s (each carrying its source `ExecutionIR`) differ only in Prompt
  Compiler rendering (e.g. wording, section order) but have identical `{NodeID, Intent,
  Acceptance, resolved TargetFiles, Constraints}`
- THEN their `SemanticHash` values are equal even though their `PromptHash` values differ
- AND a resume/retry that matches on `SemanticHash` may skip re-reasoning for that node

**Scenario: execution state changes**
- WHEN any of `{NodeID, Intent, Acceptance, resolved TargetFiles, Constraints}` changes between
  two IR compiles
- THEN `SemanticHash` differs
- AND resume/retry falls back to full re-reasoning, exactly as `PromptHash` mismatch does today

## Modified Requirements

### REQ-M01: Intent Resolver Becomes the Authoritative Target Source
> ❌ Status: Not Started (supersedes Task 1.2's fallback-only behavior from `execution-semantics-2026`)

**Scenario: Analyze step omits target_files**
- WHEN the Analyze LLM call returns an `ExecutionUnit`/`ExecutionIR` with `Capability`/`Objective`
  populated and `target_files` empty
- THEN `analyze.go` validation passes (today it fails — `analyze.go:322-332`)
- AND `ResolveIntent` runs and produces the resolved targets used for the rest of the node

**Scenario: Analyze step supplies target_files anyway**
- WHEN the Analyze LLM call returns non-empty `target_files`
- THEN `ResolveIntent` still runs (not skipped as it is today — `intent_resolver.go:97-99`'s
  early return is removed) and returns `(resolved, dropped, err)` — `dropped` holds any
  LLM-suggested path not corroborated by `analysis.AffectedFiles`
- AND the resolved set is the union of the resolver's token matches and the corroborated
  suggestions
- AND `ResolveIntent` itself performs no logging (it stays I/O-free per its documented design,
  `intent_resolver.go:90-91`); `plan.go`'s `PlanStep.Execute`, the caller that already owns
  `s.log Logger`, logs any non-empty `dropped` as a warning (observability for tuning the
  matcher, not a hard failure)

**Scenario: resolver cannot confidently resolve anything**
- WHEN neither LLM-suggested paths nor token-matching against `AffectedFiles` produce a
  confident resolution
- THEN the existing Task 1.2 behavior is unchanged: the node fails at `PLAN_READY`
  ("unresolvable intent" — `intent_resolver_test.go:TestResolveIntent_Unresolvable`)

## Removed Requirements
- None in this change set. `RunToolLoop`/legacy assembler removal is tracked under REQ-002 but
  gated on REQ-001 passing — not executed as part of this proposal's initial implementation.
