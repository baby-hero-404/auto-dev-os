# Workflow Architecture Review Report (Iteration 2)

> **Version:** v2.0
> **Source:** Workflow Logs (log2.zip)
> **Scope:** Workflow Engine, Planner, Coding Pipeline, Patch Execution, Parallel Agents
> **Assessment Date:** Current Implementation Review

---

# Executive Summary

Compared to the previous workflow, the orchestration layer has significantly improved.

The introduction of:

- Human clarification
- Human spec review
- Explicit planning
- Retry mechanism

shows that the Task State Machine is evolving toward the intended architecture.

However, the bottleneck has shifted.

The previous workflow suffered mainly from **Planning Problems**.

The current workflow is primarily limited by **Execution Engine Design**.

Most remaining issues no longer belong to OpenSpec, but to how the execution engine coordinates AI agents, validates outputs, and applies code changes.

---

# Overall Assessment

| Component | Status | Severity |
|------------|---------|----------|
| Workflow State Machine | Good | 🟢 |
| Human Gate | Implemented | 🟢 |
| Planner | Improved | 🟢 |
| OpenSpec Flow | Better | 🟠 |
| Execution Engine | Weak | 🔴 |
| Patch System | Fragile | 🔴 |
| Parallel Execution | Partial | 🟠 |
| Merge Strategy | Missing | 🔴 |
| Validation Pipeline | Partial | 🔴 |
| Observability | Needs Improvement | 🟠 |

---

# Improvements Since Previous Workflow

## Human Clarification Gate

Workflow now correctly pauses whenever additional information is required.

Expected flow

```
Context Loading

↓

Analyze

↓

Human Clarification

↓

Continue
```

This behavior matches the intended architecture.

---

## Human Spec Review

Workflow now contains

```
Human Spec Review
```

before execution.

This is a major architectural improvement.

Previously execution continued immediately after Analyze.

---

## Explicit Planning Stage

Old workflow

```
Analyze

↓

Coding
```

Current workflow

```
Analyze

↓

Plan

↓

Coding
```

Planning is now separated from implementation.

---

## Retry Mechanism

Observed

```
Apply Patch

↓

Failed

↓

Retry

↓

Success
```

Retry capability increases robustness.

However, the retry strategy remains opaque.

---

# Remaining Issues

---

# 1. Planning Is Not Persisted

Although a planning phase now exists, there is no durable planning artifact.

Missing artifacts include

- execution_plan.json
- dependency graph
- affected repositories
- execution order
- ownership mapping

Currently the planner appears to exist only during runtime.

## Impact

Planning cannot be audited.

Workflow replay becomes difficult.

Resume may generate different plans.

Priority

> 🔴 P0

---

# 2. Patch Generation Is Still The Primary Execution Format

Current

```
LLM

↓

Patch

↓

git apply
```

This architecture is inherently fragile.

Raw patches are extremely sensitive to

- whitespace
- line numbers
- file movement
- concurrent edits

Recommendation

```
LLM

↓

Structured File Edits

↓

Workspace Update

↓

Git Diff
```

Patch should be the final serialization format rather than the execution format.

Priority

> 🔴 P0

---

# 3. Patch Validation Is Missing

Observed

```
Patch

↓

git apply

↓

Failure
```

The engine should reject malformed patches before Git sees them.

Recommended pipeline

```
Patch

↓

Syntax Validation

↓

git apply --check

↓

Repair

↓

Apply
```

Priority

> 🔴 P0

---

# 4. Retry Strategy Is Not Deterministic

Current logs show retry attempts but do not explain

- why retry happened
- what changed
- whether prompts changed
- whether the patch was repaired
- whether the patch was regenerated

Retry should become explicit.

Example

```
Attempt 1

↓

Patch Invalid

↓

Repair Patch

↓

Attempt 2

↓

Validation

↓

Success
```

Priority

> 🟠 P1

---

# 5. Missing Execution Manifest

Planning exists.

Execution Manifest does not.

Current

```
Planner

↓

Coding
```

Recommended

```
Planner

↓

Execution Manifest

↓

Coding
```

Execution Manifest should define

- repository
- branch
- editable files
- commands
- constraints
- acceptance criteria
- ownership

Priority

> 🔴 P0

---

# 6. Parallel Execution Has No Dependency Graph

Multiple coding agents execute simultaneously.

No evidence exists that execution dependencies are calculated.

Potential issue

```
Agent A

modifies interface

↓

Agent B

implements old interface
```

Both complete successfully.

Merge later becomes inconsistent.

Planner should generate

```
Dependency DAG
```

before execution.

Priority

> 🟠 P1

---

# 7. Missing File Ownership

Execution appears to assign work by role only.

Missing

```
File Ownership

↓

Lock

↓

Agent Assignment
```

Without ownership,

multiple agents may modify the same file simultaneously.

Priority

> 🟠 P1

---

# 8. Missing Integration Merge Stage

Observed workflow

```
Agent

↓

Patch

↓

Apply
```

Missing

```
Merge Worktrees

↓

Conflict Detection

↓

Conflict Resolution

↓

Integration Verification
```

Merge should become an explicit orchestration phase.

Priority

> 🔴 P0

---

# 9. Planner Output Is Not Auditable

Current logs do not expose

- affected files
- execution order
- reasoning
- risks
- ownership
- selected commands

Planner decisions should become persistent artifacts.

Suggested

```
execution_plan.json

execution_graph.json

ownership.json
```

Priority

> 🟠 P1

---

# 10. Missing Semantic Validation

Current validation stops after Git Apply.

Missing semantic validation

Examples

```
Allowed Files

↓

Patch modifies forbidden file

↓

Reject
```

or

```
No Database Changes

↓

Migration generated

↓

Reject
```

Policy validation should happen before testing.

Priority

> 🔴 P0

---

# 11. Missing Patch Provenance

Every generated patch should reference

- Spec Version
- Planner Version
- Agent ID
- Prompt ID
- Execution Manifest Version

Current logs cannot reconstruct

```
Which planner output produced this patch?
```

Priority

> 🟠 P1

---

# 12. Human Approval Does Not Freeze Execution

Current

```
Human Review

↓

Coding
```

Recommended

```
Human Review

↓

Freeze Spec

↓

Compile Manifest

↓

Coding
```

Approval should freeze execution inputs.

Priority

> 🟠 P1

---

# Architecture Risks

## Runtime Planning

Planning currently appears to exist only in memory.

Any restart may produce

- different execution order
- different agent assignment
- different prompts

Planning should become deterministic.

---

## Patch-Centric Architecture

Using Git Patch as the execution primitive creates unnecessary complexity.

Modern AI coding systems typically operate on

- file edits
- AST mutations
- structured changes

and generate Git Diff only after successful execution.

---

## Weak Execution Guarantees

Current execution depends heavily on

- AI output quality
- retry behavior
- Git Apply success

The orchestration engine should instead guarantee

- deterministic workspace mutations
- deterministic validation
- deterministic merge

---

# Recommended Execution Pipeline

```
Task

↓

Context Loading

↓

Analyze

↓

Planner

↓

OpenSpec

↓

Human Review

↓

Freeze Specification

↓

Execution Manifest

↓

Dependency Graph

↓

Agent Assignment

↓

Structured File Operations

↓

Validation

    • Syntax
    • AST
    • Policy
    • Scope

↓

Workspace Update

↓

Git Diff Generation

↓

Merge

↓

Integration Tests

↓

PR Generation
```

---

# Deterministic Execution Roadmap

## P0

- Introduce Execution Manifest.
- Replace Patch-first execution.
- Add Patch Validator.
- Add Semantic Validation.
- Introduce Merge Stage.

---

## P1

- Persist Execution Plan.
- Dependency Graph.
- File Ownership.
- Planner Artifacts.
- Patch Provenance.
- Retry Visibility.

---

## P2

- Planner Visualization.
- Execution Analytics.
- Runtime Replay.
- Execution Metrics.

---

# Final Assessment

The overall architecture has clearly improved.

The planning layer is now approaching the intended design.

The next architectural bottleneck is no longer OpenSpec.

The primary weakness has moved into the execution engine.

The current execution engine still relies on AI-generated patches as the primary execution primitive, making workflow correctness dependent on LLM output quality.

To achieve production-grade reliability, the next iteration should focus on transforming the execution engine into a deterministic orchestration system where:

- AI decides **what** to change.
- The execution engine decides **how** those changes are safely applied.
- Validation occurs before any workspace mutation.
- Git becomes an output artifact rather than the execution mechanism.

This transition will substantially improve reliability, reproducibility, auditability, and multi-agent coordination while reducing hallucination-induced failures.

# Prompt Construction & Context Assembly Review

> Scope: Prompt Builder, Context Loader, Planner Prompt, Coding Prompt
> Source: Workflow Timeline + Execution Logs
> Confidence:
> - 🟢 Observed
> - 🟡 Strong Inference
> - 🔵 Architecture Risk

---

# Executive Summary

The workflow demonstrates that prompt generation is becoming increasingly modular.

However, there are still strong indications that prompt construction is tightly coupled with runtime state rather than immutable execution artifacts.

The largest remaining risks are no longer prompt quality, but prompt consistency.

Different agents may receive slightly different interpretations of the same task.

This increases hallucination, scope drift, and inconsistent implementations.

---

# 1. Prompt Is Built Dynamically Instead Of Deterministically

Confidence

🟡 Strong Inference

Observed

Workflow pauses

↓

Resume

↓

Continue

No prompt snapshot exists.

This suggests prompts are reconstructed during execution.

Potential problem

```
09:00

Prompt A

↓

Pause

↓

Human edits task

↓

Resume

↓

Prompt B
```

Now one workflow has two different contexts.

Recommendation

Persist every generated prompt.

```
planner.prompt

coder.prompt

reviewer.prompt

tester.prompt
```

Every retry should reuse the same prompt unless explicitly regenerated.

Priority

🔴 P0

---

# 2. Prompt Sources Are Not Clearly Defined

Current prompt likely includes

- Task
- Repository Context
- OpenSpec
- Workspace
- Coding Convention
- Repository Summary

There is no evidence of

```
Prompt Source Graph
```

Example

```
Task

↓

Spec

↓

Repository

↓

Architecture

↓

Manifest

↓

Prompt
```

Without explicit source ordering, different prompt builders may assemble different contexts.

Priority

🔴 P0

---

# 3. Context Priority Is Undefined

Prompt may contain

Task

Spec

Repository

Human Clarification

Acceptance

Architecture

Which one wins?

Example

Task

```
Use Redis
```

Spec

```
Don't use Redis
```

Repository Convention

```
Memory Cache Only
```

Which instruction is authoritative?

Current architecture does not appear to define prompt precedence.

Recommendation

Define

```
Execution Manifest

↓

Human Decision

↓

Spec

↓

Architecture

↓

Repository

↓

Task
```

Priority

🔴 P0

---

# 4. Prompt Is Probably Rebuilt Per Agent

Observed

Planner

↓

Backend Agent

↓

Backend Agent

↓

Reviewer

Each agent likely rebuilds context independently.

Risk

Small differences

↓

Different interpretations

↓

Different implementations

Prompt should be compiled once.

Agents should receive

Role Prompt

+

Execution Manifest

instead of reconstructing context.

---

# 5. Missing Prompt Version

No evidence exists for

```
Prompt Version

Prompt Hash

Prompt ID
```

This makes replay impossible.

If retry succeeds,

there is no way to determine

whether

- model changed
- prompt changed
- context changed

Priority

🟠 P1

---

# 6. Prompt Is Too Runtime-Oriented

Current context appears to depend heavily on

Workspace

Repository

Logs

Current State

Instead

Planner Prompt should depend primarily on

OpenSpec

Execution Manifest

Architecture Rules

Runtime data should be supplemental only.

---

# 7. Missing Context Boundary

Planner should receive

Architecture

Requirements

Repository Summary

Coder should receive

Execution Manifest

Editable Files

Coding Standards

Reviewer should receive

Diff

Acceptance

Architecture Rules

Tester should receive

Commands

Expected Outputs

Current logs suggest context is assembled independently.

Priority

🟠 P1

---

# 8. Prompt Is Missing Explicit Constraints

Good prompts should include

Allowed

Forbidden

Examples

Allowed

```
backend/auth/*
```

Forbidden

```
Docker

Terraform

CI

Deployment
```

Without constraints,

LLMs naturally expand scope.

---

# 9. Missing Prompt Validation

Before execution,

Prompt Builder should validate

Required

- Repository
- Branch
- Task
- Manifest
- Constraints

Optional

- Repository Summary
- Examples

Current workflow has no indication of prompt validation.

Priority

🟠 P1

---

# 10. Missing Prompt Snapshot

One of the biggest observability gaps.

Current logs record

Workflow State

Execution

Retry

But not

Actual Prompt

Recommended

```
logs/

planner_prompt.md

coder_prompt.md

review_prompt.md

tester_prompt.md
```

These become critical debugging artifacts.

---

# 11. Prompt Is Probably Token-Driven

Large context likely causes

Planner

↓

Repository Summary

↓

Task

↓

Logs

↓

Architecture

↓

OpenSpec

↓

LLM Context Limit

↓

Information Dropped

Instead

Prompt should be hierarchical.

```
Execution Manifest

↓

Repository Summary

↓

Relevant Files

↓

Architecture Rules

↓

Optional References
```

---

# 12. Prompt Reuse Strategy Is Missing

Every retry likely regenerates prompts.

Instead

```
Prompt V1

↓

Retry

↓

Reuse Prompt

↓

Only append failure context
```

Avoid rebuilding the entire prompt.

---

# 13. Missing Prompt Compiler

Current

Context

↓

Prompt Builder

↓

LLM

Recommended

```
Task

↓

Planner

↓

Execution Manifest

↓

Prompt Compiler

↓

Planner Prompt

Coder Prompt

Reviewer Prompt

Tester Prompt
```

Prompt Compiler becomes deterministic.

---

# 14. No Evidence Of Prompt Linting

Before sending prompts,

validate

- duplicate instructions
- conflicting constraints
- repeated repository summaries
- missing acceptance criteria
- excessive token usage

Prompt linting should become part of orchestration.

---

# 15. Prompt Provenance Is Missing

Every prompt should reference

Task ID

Spec Version

Execution Manifest Version

Planner Version

Prompt Template Version

LLM Model

Prompt Hash

Current logs cannot reconstruct

Which prompt produced this output?

---

# Overall Assessment

Prompt quality is no longer the primary issue.

Prompt consistency is.

The architecture should move from

Dynamic Prompt Assembly

to

Deterministic Prompt Compilation.

Prompt generation should become a reproducible build artifact rather than a runtime operation.

Once Prompt Compiler becomes deterministic,

multiple agents will naturally become more consistent,

hallucination will decrease,

and workflow replay will become possible.