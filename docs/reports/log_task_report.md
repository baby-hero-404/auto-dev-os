# Task System & OpenSpec Architecture Review Report

> Version: v1.0
> Scope: Task System, OpenSpec, Orchestrator, Workspace, AI Agent Execution
> Source: Workflow Logs + Task System Design Document

---

# Executive Summary

The implementation currently demonstrates a solid high-level architecture, but several critical execution behaviors deviate from the intended design.

The most important finding is that **OpenSpec is not functioning as the execution contract of the system**.

Instead, agents appear to continue relying on the original task description and repository context, leading to semantic inconsistencies and orchestration failures.

The issue observed with `zentao-tool` is **not an isolated Go error**, but a symptom of a deeper architectural problem involving metadata propagation and execution contracts.

---

# Overall Assessment

| Area | Status | Severity |
|-------|---------|----------|
| Task State Machine | Partial | 🔴 High |
| OpenSpec | Partial | 🔴 High |
| Planner Pipeline | Partial | 🟠 Medium |
| Execution Contract | Missing | 🔴 High |
| Workspace | Good Foundation | 🟠 Medium |
| Resume Strategy | Incomplete | 🟠 Medium |
| Logging & Tracing | Weak | 🟠 Medium |
| Multi Repo Design | Good | 🟢 Low |
| AI Context Management | Needs Improvement | 🔴 High |

---

# Critical Findings

## 1. OpenSpec is not the Source of Truth

### Expected

```
Task
    ↓
Planner
    ↓
OpenSpec
    ↓
Human Approval
    ↓
Execution
```

### Observed

```
Task
    ↓
Planner
    ↓
Code Agent
```

OpenSpec appears to be generated as documentation rather than becoming the execution contract.

### Impact

- Planner and Coder may interpret the task differently.
- Human edits to OpenSpec may never affect execution.
- Agents continue relying on raw task descriptions.

Priority:

> 🔴 P0

---

# 2. Semantic Metadata Mixing

Observed symptom:

```
change_name = zentao-tool
```

Later becomes

```
go test ./zentao-tool/...
```

This indicates metadata confusion.

Current concepts:

- Task Name
- Change Name
- Repository
- Go Module
- Workspace Path

appear to be mixed together.

These concepts must never be interchangeable.

### Root Cause

Likely semantic mapping issue.

Instead of

```
change_name
```

being used only for

```
openspec/changes/<change_name>
```

it is later interpreted as

- repository
- module
- package
- test path

This is an architectural bug.

Priority:

> 🔴 P0

---

# 3. Workflow Continues After Analyze Failure

Observed

```
Analyze Failed

↓

Coding

↓

Testing

↓

Diff Generation
```

Expected

```
Analyze Failed

↓

FAILED
```

Execution should never continue without a valid plan.

Priority

> 🔴 P0

---

# 4. Missing OpenSpec Validation

Generated OpenSpec is never validated before execution.

Missing checks include:

- proposal exists
- specs exists
- tasks exists
- design exists
- yaml valid
- task list valid
- acceptance criteria valid

Planner output should never directly reach coding.

Priority

> 🔴 P0

---

# 5. Missing Execution Contract

Current workflow

```
Task

↓

Planner

↓

Markdown

↓

Agent
```

Recommended

```
Task

↓

Planner

↓

OpenSpec

↓

Execution Manifest

↓

Code Agent
```

Markdown is intended for humans.

Execution requires structured machine-readable data.

Priority

> 🔴 P0

---

# OpenSpec Issues

## OpenSpec Has Too Many Responsibilities

Current

- Documentation
- Planning
- Resume
- Audit
- Execution

These should be separated.

Recommended

```
Human Documents

proposal.md

design.md

tasks.md

↓

Execution Manifest (JSON)

↓

Agents
```

---

## Planner Does Not Freeze Scope

Planner generates a specification.

However scope is never frozen.

Different agents may interpret different scopes.

Planner

```
Implement Login
```

Coder

```
OAuth Login
```

Reviewer

```
JWT Login
```

All are technically valid.

Scope should become immutable after approval.

---

## Missing Spec Version

No evidence of

```
Spec V1

↓

Human Edit

↓

Spec V2

↓

Approved

↓

Execution
```

Without versioning:

Resume becomes unreliable.

---

## Missing Spec Hash

Approved specifications should be immutable.

Suggested

```
Spec Version

SHA256

Approval Time

Approved By
```

Execution should always reference this hash.

---

## Missing Contract Validation

Planner output should be validated before execution.

Validation should include

- Repository
- Test Strategy
- Acceptance Criteria
- Risks
- Constraints
- Files
- Dependencies

Only then should Coding begin.

---

# Task Model Issues

Current Task

```
Title

Description
```

Recommended

```
Objective

Scope

Out Of Scope

Constraints

Affected Repositories

Affected Modules

Acceptance Criteria

Risk Domains

Dependencies
```

Task should become semantic instead of plain text.

---

# Workspace Issues

The workspace design is good but several execution guarantees are missing.

## Missing Checkpoint Visibility

Current logs never indicate

```
Checkpoint Saved

Checkpoint Restored

Resume From
```

Resume cannot be audited.

---

## Too Many Sources of Truth

Current

- Database
- task.json
- metadata.json
- OpenSpec
- Context
- Workspace
- Prompt

These may drift apart.

Recommended

Only one execution contract should exist.

---

## Workspace Stores Too Much Mutable State

Workspace currently stores

- task
- metadata
- context
- specs
- checkpoints
- logs

Some of these should become immutable artifacts.

---

# AI Agent Issues

## Planner and Coder Are Loosely Coupled

Current

```
Planner

↓

Markdown

↓

Coder
```

Likely implementation

```
Planner

↓

Task Description

↓

Coder
```

Instead

```
Planner

↓

Execution Manifest

↓

Coder
```

---

## Missing Execution Boundary

Planner should explicitly define

Allowed

```
backend/

repository/
```

Forbidden

```
docker/

ci/

deployment/
```

This prevents over-editing.

---

## Acceptance Criteria Are Not Machine Readable

Current

```
Developer can create tasks.
```

Recommended

```json
{
  "acceptance": [
    {
      "id":"AC-1",
      "type":"api",
      "endpoint":"/tasks",
      "status":201
    }
  ]
}
```

---

## Missing Dependency Graph

tasks.md is sequential.

Execution needs

```
Migration

↓

Repository

↓

Service

↓

API

↓

Frontend
```

Dependencies should be explicit.

---

## Context Is Too Large

Current context may include

- architecture
- repository
- task
- spec
- logs
- conventions

Each role should receive only relevant information.

Planner

↓

Architecture

Coder

↓

Execution Manifest

Reviewer

↓

Diff + Acceptance

Tester

↓

Commands + Expected Results

---

# Observability Issues

Current logs lack

- Analyze reason
- Retry count
- Step duration
- Parent step
- Correlation ID
- Execution ID
- Spec Version

Logging should become structured.

---

# Recommended Architecture

```
Task
    │
    ▼
Planner
    │
    ▼
OpenSpec
    │
    ▼
Validation
    │
    ▼
Human Approval
    │
    ▼
Freeze
    │
    ▼
Execution Manifest
    │
    ▼
Planner
    │
    ▼
Coder
    │
    ▼
Reviewer
    │
    ▼
Tester
    │
    ▼
PR
```

Execution Manifest becomes the only source of truth.

---

# Priority Roadmap

## P0

- OpenSpec becomes execution contract.
- Separate metadata concepts.
- Add Execution Manifest.
- Fail Fast after Analyze.
- Validate OpenSpec before Coding.
- Inject repository/module information.
- Freeze approved specifications.

---

## P1

- Structured logging.
- Spec Versioning.
- Spec Hash.
- Correlation IDs.
- Resume improvements.
- Checkpoint visibility.

---

## P2

- Machine-readable Acceptance Criteria.
- Dependency Graph.
- Role-specific Context.
- Semantic Task Model.
- AI Context Optimization.

---

# Root Cause Summary

The observed failures are not primarily caused by AI code generation.

The deeper issue is that the orchestration layer lacks a single immutable execution contract.

Current execution appears to rely on multiple mutable sources:

- Task Description
- Repository Context
- Generated Markdown
- Workspace Metadata

This creates semantic inconsistencies across Planner, Coder, Reviewer and Tester.

The `zentao-tool` incident is therefore best understood as a symptom of metadata ambiguity rather than a Go testing issue.

---

# Final Recommendation

The highest priority architectural improvement is to transform OpenSpec from documentation into an immutable execution contract.

Every downstream agent should execute exclusively from a validated, versioned Execution Manifest.

Doing so will significantly reduce:

- Hallucinated repository paths
- Incorrect module inference
- Scope drift
- Planner/Coder inconsistency
- Resume failures
- Cross-agent semantic mismatches
- Workflow instability