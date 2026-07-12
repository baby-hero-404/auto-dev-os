# Deep Investigation Report

# LLM Call Pipeline & Prompt Architecture Review

> Version: v2.0 (Code-Verified)
> Scope:
>
> - LLM Call Lifecycle
> - Prompt Architecture
> - Agent Responsibilities
> - Prompt Contract
> - Tool Calling Policy
> - Multi-Agent Coordination
>
> **Analysis Method**
>
> This report has been **verified against the actual source code** of Auto-Dev-OS:
>
> - `server/internal/prompts/builder.go` — Prompt Assembly Engine
> - `server/internal/prompts/assembler.go` — Section Collection & Budget Optimization
> - `server/internal/orchestrator/steps/coding_instruction.go` — Coding Instruction Builder
> - `server/internal/orchestrator/steps/frozen_context.go` — FrozenContext Propagation
> - `server/internal/orchestrator/steps/boundary_tool_executor.go` — Tool Policy Enforcement
> - `server/internal/orchestrator/llmrunner/toolloop.go` — Agentic Tool Loop
> - `server/internal/workflow/step.go` — DAG Workflow Definitions
> - `server/internal/prompts/steps/*.md` — Step Prompts
> - `server/internal/prompts/roles/*.md` — Role Prompts
> - `server/internal/prompts/core/*.md` — Core System Prompts
>
> Each finding below is tagged with its **actual evidence** from the codebase.

---

# Executive Summary

The Auto-Dev-OS prompt architecture is **significantly more mature** than the original report suggested. Many findings that were labeled "Confirmed" were actually incorrect or outdated assumptions based on incomplete log analysis.

## What the system ALREADY does well

1. **FrozenContext** — The Plan step produces a `FrozenContext` snapshot that is injected into all downstream agents. This IS a form of execution contract (`frozen_context.go`).
2. **Execution Manifest (JSON)** — Coding steps receive a structured JSON manifest containing `affected_files`, `tasks`, `acceptance_criteria`, and `execution_boundaries` (`builder.go:636-671`).
3. **Execution Boundaries** — The `BoundaryCheckedToolExecutor` enforces file-level policy with severity escalation (Critical → Pause, Error → Feedback, Warning → Auto-expand) (`boundary_tool_executor.go`).
4. **Context Budget Optimization** — Prompt sections have priority scores. When the token budget is exceeded, mutable sections are dropped by priority (`builder.go:824-884`).
5. **Semantic Context Deduplication** — Snippets are deduplicated and filtered by affected files for coding steps (`builder.go:716-719`).
6. **Spec Omission for Coding Steps** — ProposalMD, SpecsMD, and DesignMD are explicitly excluded from coding prompts (`builder.go:613-624`). Only TasksMD and the Execution Manifest are forwarded.

## What still needs improvement

The primary architectural bottleneck is **prompt phase clarity** — agents lack explicit "you are in phase X" framing, causing LLM models to restart reasoning from scratch instead of consuming deterministic decisions.

---

# Architecture Overview

## Actual Workflow DAG (Verified)

```text
context_load → analyze → plan → code_backend_0..N ──┐
                                 code_frontend_0..N ─┤
                                                     ├→ merge → review → fix → test → pr
```

**Source:** `workflow/step.go` — `MediumWorkflow()`, `DynamicDAGWorkflow()`

The system supports 4 workflow topologies:
- `EasyWorkflow` — context → analyze → code → test → pr (no plan/review)
- `MediumWorkflow` — full pipeline with sequential subtasks
- `HardWorkflow` — alias for MediumWorkflow
- `DynamicDAGWorkflow` — LLM-generated DAG from `ExecutionUnits` with dependency & parallelism constraints

## Actual Prompt Assembly Order (Verified)

```text
System Prompt = [
  Base Prompt        (core/system_prompt.md)         — Priority 10, Immutable
  Role Prompt        (roles/coder.md)                — Priority 20, Immutable
  Step Prompt        (steps/code_backend.md)         — Priority 50, Mutable
  JIT Skills         (resolved from RequiredSkillsMap) — Priority 60, Mutable
  Global Rules       (from rules database)            — Priority 15, Immutable
  Role Constraints   (role-scoped rules)              — Priority 25, Immutable
  Project Rules      (strict/advisory)                — Priority 35/45
  Output Rules       (core/output_rules.md)           — Priority 35, Immutable
  Available Tools    (JSON schema of all tools)       — Priority 5, Immutable
]

User Prompt = [
  Task Requirement   (title + description OR spec omission notice)  — Priority 30, Immutable
  Clarifications     (Q&A rounds)                     — Priority 70, Mutable
  Task Specifications (OpenSpec — only for analyze/plan/context_load) — Priority 40, Mutable
  Execution Manifest (JSON — affected_files, tasks, boundaries)     — Priority 40, Mutable
  Relevant Specs     (subset for coding steps)        — Priority 40, Mutable
  Tasks Progress     (for indexed subtasks)           — Priority 80, Mutable
  Semantic Context   (code snippets)                  — Priority 100, Mutable
  Repository Map     (tree structure)                 — Priority 100, Mutable
  Memories           (episodic memory)                — Priority 90, Mutable
]
```

**Source:** `builder.go:400-793`

---

# Finding 1

## Agent Responsibilities Overlap Is PARTIALLY True

**Confidence:** 🟡 Partially Confirmed

### What the report originally claimed
> The Coding Agent receives instructions to understand repository, inspect architecture, validate assumptions.

### What the code actually shows

The **Role Prompt** (`roles/coder.md`) does contain:
```
If modifying existing files, carefully review the code context and repo map.
```

However, this is NOT a planning instruction. It says "review the context **already provided to you**", not "go discover the architecture yourself."

The **Step Prompt** (`steps/code_backend.md`) is narrow:
```
Implement backend endpoints, models, database migrations, and business logic.
```

The **Coding Instruction Template** (`coding_instruction.tmpl`) provides:
- Repository structure (tree) — pre-scanned, NOT requiring the agent to discover it
- Assigned subtasks — from Plan output
- Prior files from other steps — explicit list

### Actual Issue
The overlap exists primarily because:
1. The `coder.md` role prompt says "review the code context and repo map" which can be misinterpreted by the LLM as "go explore the repo"
2. The system provides semantic code snippets AND a directory tree AND an execution manifest — the LLM may try to reconcile all three instead of just following the manifest

### Evidence
- `coding_instruction.go:86-105` — Tree is pre-scanned, not requiring agent exploration
- `coding_instruction.go:108-128` — Subtasks are injected from Plan output
- `builder.go:609-687` — Coding steps get compact context, NOT full OpenSpec

---

# Finding 2

## Planner Output — REFUTED: It DOES Produce Structured Decisions

**Confidence:** 🔴 Refuted

### What the report originally claimed
> Planner produces knowledge instead of decisions (repository summaries, architectural descriptions).

### What the code actually shows

The Plan step (`steps/plan.go`) produces:
1. **FrozenContext** — A complete snapshot of all execution decisions (`frozen_context.go:28-41`):
   - `SpecHash`, `ProposalMD`, `SpecsMD`, `DesignMD`, `TasksMD`
   - `ExecutionUnits` (DAG nodes with agent assignments, dependencies, constraints)
   - `ExecutionBoundaries` (file-level policy with root dirs and capabilities)
   - `AffectedFiles` (exact file list with action type: modify/create/delete)
   - `AcceptanceCriteria` (pass/fail conditions)
   - `ExecutionPhases`, `Risks`, `RiskDomains`

2. **Subtask assignments** — `subtasks` map keyed by `backend`/`frontend` with ordered task lists

3. **Execution Manifest (JSON)** — Injected into Coding prompts (`builder.go:636-671`) containing:
   ```json
   {
     "affected_files": [...],
     "tasks": [...]
   }
   ```

The Planner IS producing deterministic execution decisions, not just documentation.

### Remaining Issue
The `plan.md` step prompt still says:
```
Analyze the codebase structure, draft specifications, and output the proposed plan matching the execution architecture.
```
The phrase "Analyze the codebase structure" is redundant because the Analyze step has already done this. The Plan step prompt should say "Consume the analysis output and produce execution decisions."

---

# Finding 3

## Coding Prompt Is Execution-Centric — PARTIALLY Refuted

**Confidence:** 🟡 Partially Confirmed

### What actually happens

For **coding steps** (`isCodingStep(stepID) == true`), the PromptAssembler explicitly OMITS:
- `ProposalMD` (`builder.go:613`)
- `SpecsMD` (`builder.go:617`)
- `DesignMD` (`builder.go:621`)

It only injects:
- `TasksMD` or structured `Tasks` list
- Execution Manifest (JSON) with `affected_files` and `tasks`
- Relevant Requirements subset for the specific subtask index
- Tasks Progress (what's done vs. what's pending)

### Remaining Issue
The `coding_instruction.tmpl` template still injects a full `Repository Structure` tree scan (up to 200 entries, depth 3). While this is pre-scanned (not requiring agent exploration), it adds unnecessary tokens when the Execution Manifest already specifies exact file targets.

### Evidence
- `coding_instruction.go:97-105` — `ScanDirectory(physicalRoot, 3, 200)` always runs
- `builder.go:675-687` — Per-subtask spec extraction and progress summary

---

# Finding 4

## Shared Working Memory — EXISTS (FrozenContext)

**Confidence:** 🔴 Refuted

### What the report originally claimed
> Each agent reconstructs knowledge independently. No shared working memory exists.

### What the code actually shows

The system HAS a shared working memory mechanism:

1. **FrozenContext** — Snapshot from Plan step, propagated to ALL downstream agents via `StepInputs` context (`frozen_context.go`)
2. **ContextCache** — Built by `ContextLoadStep`, containing `SemanticSnippets`, `RepoMap`, `DirectoryTree` — shared across all steps (`builder.go:690-700`)
3. **Step Inputs** — Each step receives outputs from its DAG dependencies (`workflow/engine.go:189`)
4. **Prior Files** — Files modified by `code_backend_*` steps are tracked and injected into `code_frontend_*` prompts (`coding_instruction.go:130-154`)

### Remaining Issue
While the data IS shared, it is **re-serialized into prompt text** at each step. There is no binary/structured memory object that agents can query. Each agent receives a different text rendering of the same data, which introduces formatting variance.

---

# Finding 5

## Prompt Does Not Explicitly Encode Workflow Phase — CONFIRMED

**Confidence:** 🟢 Confirmed

### Evidence

The `system_prompt.md` says:
```
You are an AI Agent operating within the Auto Code OS framework.
```

The `code_backend.md` says:
```
You are a Backend Specialist. Your goal is to implement...
```

Neither prompt says:
```
Current Phase: IMPLEMENTATION
Planning has completed. Architecture has been approved.
Only implement the supplied execution plan.
```

The `assembler.go:156-161` has a `shouldInjectFullSpec()` function that distinguishes phases, but this logic is used to **filter what data is injected**, not to **tell the agent what phase it is in**.

### Impact
Without explicit phase framing, LLMs (especially instruction-following models) may interpret "implement backend changes" as "first understand what needs to change, then implement" rather than "the plan already tells you exactly what to change, just do it."

---

# Finding 6

## Missing Explicit Success Criteria — PARTIALLY REFUTED

**Confidence:** 🟡 Partially Confirmed

### What the code actually shows

The system DOES define acceptance criteria:
- `AcceptanceCriteria` in `FrozenContext` is propagated to coding agents via the Execution Manifest (`builder.go:660-662`)
- The `coder.md` role prompt says: "Your implementation MUST fulfill the `acceptance_criteria` defined in the Execution Manifest."

### Remaining Issue
The acceptance criteria are in the **user prompt** as part of a JSON blob. They are NOT prominently displayed as "this iteration succeeds if X" at the top of the prompt. The LLM may not treat them as hard exit conditions.

Additionally, there is no per-iteration success criterion like:
```
This iteration is complete when you have:
1. Made all file changes specified in affected_files
2. Called the summary tool with your changes
```

---

# Finding 7

## No Hard Exit Conditions for Exploration — CONFIRMED

**Confidence:** 🟢 Confirmed

### Evidence

The `toolloop.go` has a `MaxIterations` budget (default 8), but this is a **hard ceiling**, not a directed policy. There is no prompt instruction that says:
```
Stop exploring when you have identified your implementation targets.
Read at most 5 files before starting to write code.
```

The `coder.md` role prompt says "review the code context and repo map" but provides no stopping condition for when that review should end.

### Evidence
- `toolloop.go:49-50` — `MaxIterations` defaults to 8
- No prompt text found containing exploration limits

---

# Finding 8

## Tool Calling Policy — PARTIALLY CONFIRMED

**Confidence:** 🟡 Partially Confirmed

### What the code actually shows

The system DOES have tool-level policy enforcement:
1. **BoundaryCheckedToolExecutor** — `search_replace` and `create_file` are checked against execution boundaries (`boundary_tool_executor.go:19-22`)
2. **Critical violations** pause the task immediately (`boundary_tool_executor.go:52-53`)
3. **Error violations** are fed back to the LLM as corrective feedback (`boundary_tool_executor.go:54-55`)
4. **JIT Skills** can specify `allowed-tools` per skill, filtering the tool set

### Remaining Issue
Read-only tools (`read_file`, `list_files`, `grep_search`) have NO policy constraints. The LLM can:
- Read the same file multiple times
- Explore directories outside its assigned module
- Run unlimited search queries

There is no token-cost awareness or duplicate-read detection.

### Evidence
- `boundary_tool_executor.go:19-22` — Only `search_replace` and `create_file` are boundary-checked
- `builder.go:562-565` — All tools are injected without read-limiting policy

---

# Finding 9

## Missing Negative Instructions — CONFIRMED

**Confidence:** 🟢 Confirmed

### Evidence

The `coder.md` role prompt contains:
```
Do NOT rewrite or re-architect the entire system unless specifically requested.
```

This is the ONLY negative instruction. Missing:
```
Do NOT re-analyze the repository structure if it has already been provided.
Do NOT explore files outside your assigned affected_files list.
Do NOT read a file more than once per session.
Do NOT run tests yourself — the orchestrator will run them automatically after you finish.
```

The `coding_instruction.tmpl` adds:
```
DO NOT rewrite the entire file unless creating a new file.
```

But there are no negative constraints about exploration behavior.

---

# Finding 10

## Planner Output Is Not Executable — REFUTED

**Confidence:** 🔴 Refuted

As shown in Finding 2, the Planner produces:
- `ExecutionUnits` with explicit `agent`, `dependencies`, `constraints.parallelizable`
- `ExecutionBoundaries` with `root` dirs and `capabilities` list
- `AffectedFiles` with `file`, `action`, `reason`
- `AcceptanceCriteria` as structured list

This IS an execution contract. The Coding Agent consumes it via the JSON Execution Manifest.

---

# Finding 11

## Agents Optimize Correctness Rather Than Progress — CONFIRMED

**Confidence:** 🟢 Confirmed

### Evidence

The `coder.md` role prompt emphasizes:
```
Write clean, efficient, and well-tested code.
Follow existing project conventions and architectures.
Carefully review the code context and repo map.
```

No prompt text contains urgency like:
```
Prioritize forward progress over maximum confidence.
Implement first, then verify. Do not pre-verify.
```

The system prompt says:
```
Only write code that is clean, tested, and correct.
```

The word "tested" before "correct" implies the agent should verify before outputting, encouraging exploration loops.

---

# Finding 12

## No Confidence Threshold — CONFIRMED (Architectural)

**Confidence:** 🔵 Confirmed as Architectural Gap

No prompt text contains confidence thresholds. The LLM has no guidance on when "enough information" has been gathered. This is an architectural design choice — there is no mechanism to implement this without prompt changes.

---

# Finding 13

## Planner Output Is Not Compressed — PARTIALLY CONFIRMED

**Confidence:** 🟡 Partially Confirmed

### What the code actually shows

For **coding steps**, the system DOES compress:
- Full `ProposalMD`, `SpecsMD`, `DesignMD` are OMITTED (`builder.go:613-624`)
- Only `TasksMD` + Execution Manifest JSON are forwarded
- Per-subtask spec extraction narrows context to relevant sections (`builder.go:675-687`)

### Remaining Issue
The `TasksMD` can still be quite large (full task breakdown for all subtasks, not just the assigned one). The system does extract the relevant section via `extractSpecsSectionForSubtask()`, but the full manifest JSON still includes ALL affected files, not just the ones relevant to the current subtask.

---

# Finding 14

## Prompt Contracts Are Too Broad — PARTIALLY CONFIRMED

**Confidence:** 🟡 Partially Confirmed

### What the code actually shows

Each role has a dedicated prompt file:
- `coder.md` — "implement the exact specifications"
- `reviewer.md` — "inspect code changes for requirements conformance"
- `planner.md` — planning-specific instructions

### Remaining Issue
The `code_backend.md` step prompt says:
```
1. Implement secure, high-performance Go, Node.js, or Python code.
2. Ensure database operations are optimized and follow standard schema migrations.
3. Write clean, modular, and self-documenting code.
4. Adhere strictly to clean coding standards and the specified execution boundaries.
```

Point 2 asks the agent to "ensure database operations are optimized" — this is a review/optimization concern, not an implementation concern. The contract should say "implement the database changes specified in your assigned subtask."

---

# Finding 15

## Semantic Reconstruction — PARTIALLY CONFIRMED

**Confidence:** 🟡 Partially Confirmed

### What the code actually shows

The `FrozenContext` is transferred as structured JSON between steps. This is NOT just "prompt text" — it's a typed Go struct serialized to JSON and deserialized by each downstream step.

However, when it reaches the LLM, it IS rendered as prompt text. The LLM must re-parse the text to understand the decisions.

### Actual Architecture

```
Analyzer → TaskAnalysis (Go struct in DB)
              ↓
Planner  → FrozenContext (Go struct, JSON serialized)
              ↓
Builder  → Selectively renders relevant sections as prompt text
              ↓
Coder    → Reads prompt text, executes tools
```

The semantic variance is introduced at the **Builder → Coder** boundary, not at every step transition.

---

# Corrected Architectural Summary

## What IS Working

| Feature | Status | Evidence |
|---------|--------|----------|
| Structured execution contract | ✅ Exists | `FrozenContext`, `ExecutionManifest` |
| File-level boundary enforcement | ✅ Exists | `BoundaryCheckedToolExecutor` |
| Context budget optimization | ✅ Exists | `optimizeBudget()` with priority-based pruning |
| Spec omission for coding steps | ✅ Exists | `builder.go:613-624` |
| JIT skill resolution | ✅ Exists | `resolveSkills()` with scoring |
| Role-scoped rules | ✅ Exists | `filterRulesForAgent()` |
| Dynamic DAG topology | ✅ Exists | `DynamicDAGWorkflow()` |

## What NEEDS Improvement

| Issue | Severity | Fix Complexity |
|-------|----------|----------------|
| No phase framing in prompts | 🔴 High | Low — add 3 lines to coding step prompts |
| No exploration limits for read-only tools | 🟡 Medium | Medium — add prompt instructions + optional tool-call counter |
| Missing negative instructions | 🟡 Medium | Low — add 5-6 lines to `coder.md` |
| Step prompts too generic | 🟡 Medium | Low — rewrite `code_backend.md` and `plan.md` |
| `TasksMD` not scoped to assigned subtask | 🟠 Low-Med | Medium — filter by subtask index in builder |
| Tree scan always runs even when manifest exists | 🟠 Low | Low — conditional in `coding_instruction.go:97` |

---

# Priority Recommendations

## 1. Add Phase Framing to Coding Prompts (HIGH IMPACT, LOW EFFORT)

Update `steps/code_backend.md`:
```markdown
# Step: Code Backend

## Current Phase: IMPLEMENTATION

Planning and analysis have already completed.
Architecture decisions have been finalized.
Your execution contract is provided in the Execution Manifest below.

## Your ONLY Responsibility
Implement the exact changes specified in your assigned subtask.
Do NOT re-plan, re-analyze, or re-design.
```

## 2. Add Negative Instructions to `roles/coder.md` (HIGH IMPACT, LOW EFFORT)

Append:
```markdown
# Prohibited Actions
- Do NOT explore files outside your assigned affected_files list unless a compilation error requires it.
- Do NOT read a file more than once per session.
- Do NOT re-analyze repository architecture — this has already been done.
- Do NOT run tests yourself — the orchestrator handles test execution automatically.
- Do NOT suggest alternative architectures or redesigns.
```

## 3. Add Exploration Budget to Tool Loop (MEDIUM IMPACT, MEDIUM EFFORT)

In `toolloop.go`, track read-only tool call counts and inject a warning message when the budget is 75% consumed:
```
Warning: You have used 6 of 8 iterations. Focus on producing your final output now.
```

## 4. Skip Tree Scan When Manifest Exists (LOW IMPACT, LOW EFFORT)

In `coding_instruction.go:97-105`, skip `ScanDirectory()` when the Execution Manifest already provides `affected_files` with specific file paths.

---

# Next Steps & Deep Dive Action Items

Based on the investigation above, 5 high-risk areas have been identified for further remediation. These have been formalized into an OpenSpec execution plan to systematically address prompt architecture and orchestration resilience.

## Identified Risks

1. **Subtask Routing Fragility:** `isRole` fallback logic in `extractSpecsSectionForSubtask` is non-deterministic.
2. **Budget Optimization Data Loss:** `Execution Manifest` may be dropped under tight token budgets.
3. **Type Assertion Crash:** `code_backend.go` risks a runtime panic when parsing plan outputs.
4. **Sandbox Commit Data Loss:** Committing *after* the agent loop completes risks losing work on git failures.
5. **Stale Review Context:** Reviewers do not currently receive the `FrozenContext`.

## OpenSpec Reference
The complete specification and task breakdown can be found at:
- **Proposal:** [proposal.md](file:///home/tiger/my_projects/auto-dev-os/docs/openspecs/orchestrator-resilience/proposal.md)
- **Specs:** [specs.md](file:///home/tiger/my_projects/auto-dev-os/docs/openspecs/orchestrator-resilience/specs.md)
- **Tasks:** [tasks.md](file:///home/tiger/my_projects/auto-dev-os/docs/openspecs/orchestrator-resilience/tasks.md)
