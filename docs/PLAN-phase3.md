# Phase 3 Implementation Plan — Orchestrator + Agent Manager + Sandbox + Workflow Engine

> **Status:** 📋 Planned
> **Depends on:** Phase 2 (Git Integration + Web UI + Auth)
> **Structure:** Split into **Phase 3a** (Sandbox + Agent Manager + Orchestrator Core) and **Phase 3b** (Workflow Engine + Prompt Assembly + Execution UI) to reduce risk.

**Goal:** Build the core AI execution pipeline — assign tasks to agents, run them in isolated Docker sandboxes, and orchestrate the full workflow (plan → code → review → fix → test → PR).

---

## References

> Study these resources before starting implementation.

### Learning Report — `resources/Learning_Report.md`

| Section | Key Learnings for Phase 3 |
|---------|---------------------------|
| §2 OpenClaw | **Sandboxing** — Docker/SSH/OpenShell isolation patterns, security model with defined permissions |
| §3 AI-SDLC | **Orchestrator pipeline** — WATCH→ASSESS→ROUTE→EXECUTE→VALIDATE→DELIVER→LEARN loop, Worktree Isolation (Pattern-C) |
| §5 OpenSpec | **Parallel Execution & Merging** — running agents on different modules in parallel, merge strategy |
| §7 Superpowers | **TDD workflow** — RED-GREEN-REFACTOR enforcement, Subagent-Driven Development, two-step review |
| §8 Hermes Agent | **Delegation & Parallelization** — spawning independent sub-agents, multi-step RPC pipelines |

### Reference Doc — `resources/Reference_doc.md`

| Section | Key Learnings for Phase 3 |
|---------|---------------------------|
| §2.1 AI-SDLC | **Cross-Harness Review** — independent review by different AI harnesses (Claude + Codex), autonomy ledger |
| §2.1 AI-SDLC | **Governance (DoR)** — Definition of Ready gates before agent execution starts |
| §2.2 OpenClaw | **Flexible Sandboxing** — multiple sandbox backends (Docker, SSH, OpenShell) |
| §2.3 Multica | **Agent lifecycle management** — task assignment, progress tracking, skill reuse |
| §3.2 | **Orchestration model** — sequential, hierarchical, and group chat agent coordination |
| §3.5 | **CI/CD Feedback Loop** — auto-create fix tasks on CI failure |

### Deep Code References in `resources/`

| Component | Path to Study | What to Learn / Reuse |
|-----------|---------------|-----------------------|
| **Orchestrator Pipeline** | `resources/ai-sdlc/orchestrator/src/` | Study the state machine steps: `watch.ts` (trigger), `triage.ts` (assign), `execute.ts` (code), `review.ts` (AI review), `fix-review.ts` (auto-fix), `fix-ci.ts` (CI feedback loop). |
| **Agent Personas** | `resources/prompt_base/antigravity/agents/` | 14 specialist agent markdown definitions (Orchestrator, Planner, Backend Specialist, etc.) to use as system prompts. |
| **Rules & Governance** | `resources/prompt_base/core/` | Use `rules.md`, `classifier.md`, and `system_prompt.md` as templates for the Prompt Assembly (Rule Engine) step. |
| **Sandboxing** | `resources/openclaw/packages/openclaw-sandbox/` | Review how OpenClaw creates isolated Docker environments and streams shell output. |
| **Task Assignment** | `resources/multica/server/` | Go-based orchestrator patterns for assigning tasks to agents based on skills. |
| **TDD Workflow** | `resources/superpowers/skills/tdd/` | Markdown-based instructions for agents to strictly follow Red-Green-Refactor. |
| **Parallel Execution** | `resources/OpenSpec/openspec-parallel-merge-plan.md` | Design document for running multiple agents on different modules concurrently and merging their work. |

---

## ⚠️ Pre-requisite: Human Review Gate

> **MANDATORY:** Before starting Phase 3, the team must review:
> 1. All Phase 2 deliverables are verified and tested.
> 2. `docs/ARCHITECTURE.md` — Confirm orchestrator and sandbox design decisions.
> 3. The reference code paths listed below — understand sandbox isolation, orchestration pipelines, and parallel execution patterns.
> 4. `resources/OpenSpec/schemas/spec-driven/schema.yaml` — Review spec-driven I/O schemas for workflow step validation.
>
> **Only proceed after the team signs off.**

---

# Phase 3a — Sandbox + Agent Manager + Orchestrator Core

> **Goal:** Build the execution infrastructure — isolated sandbox containers, agent pool management, and basic sequential orchestration.

---

## Task 1: Sandbox Runtime (Docker) & Network Isolation

**Files:**
- Create: `server/internal/sandbox/sandbox.go` — Docker container lifecycle
- Create: `server/internal/sandbox/workspace.go` — worktree management
- Create: `server/internal/sandbox/policy.go` — action interceptor
- Create: `docker/Dockerfile.sandbox` — base image for agent execution

**Scope:**
- [ ] Create & destroy Docker containers per task
- [ ] Mount git worktree (cloned repo) as volume
- [ ] Execute commands inside container (git, tests, build)
- [ ] Capture stdout/stderr output and stream to API
- [ ] Resource limits (CPU, memory, timeout)
- [ ] **Network Isolation**: Disable default internet access. Route traffic through an internal proxy to monitor/block unauthorized outbound connections.
- [ ] **Policy Engine**: Enforce declarative policies (e.g., block `rm -rf`, restrict write access to specific directories) by intercepting shell commands before execution.

---

## Task 2: Secret Vault Management

**Files:**
- Create: `server/migration/000005_secrets.up.sql` — AES encrypted storage
- Create: `server/internal/service/secrets.go`

**Scope:**
- [ ] Store project-specific environment variables securely (AES-GCM encryption).
- [ ] Retrieve and decrypt secrets at runtime.
- [ ] Inject secrets directly into the Sandbox container as secure ENV vars.
- [ ] Prevent secrets from appearing in agent logs or SSE streams.

---

## Task 3: Agent Manager & Job Queue

**Files:**
- Create: `server/internal/orchestrator/agent_manager.go`
- Create: `server/internal/orchestrator/queue.go`

**Scope:**
- [ ] **Job Queue**: Implement PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED` for robust background task queuing.
- [ ] Agent pool — track idle/busy agents
- [ ] Agent assignment — match task complexity + role to agent capabilities
- [ ] Concurrent agent execution — goroutine per agent task, pulling from queue.
- [ ] Agent status lifecycle: `idle` → `assigned` → `running` → `idle`

---

## Task 4: Orchestrator Core & Checkpointing

**Files:**
- Create: `server/internal/orchestrator/orchestrator.go`

> **Aligns with:** `docs/manual/Roadmap.md` §2 — Complexity-based branching (Easy/Medium/Hard)

**Scope:**
- [ ] Receive task from API → trigger analysis by Planner agent
- [ ] **Complexity-based branching** (implements Roadmap §2):
  - **Easy tasks** (`spec_status = AUTO_APPROVED`): Skip human review → assign agent → dispatch to sandbox immediately.
  - **Medium/Hard tasks** (`spec_status = PENDING_REVIEW`): Pause execution → wait for human approval (`spec_status = APPROVED`) before dispatching to sandbox.
  - **Clarification loop**: If agent needs more info, set `spec_status = CHANGES_REQUESTED`, emit clarification questions via SSE, and wait for developer response before re-analyzing.
- [ ] Task state machine transitions via service layer (TODO → ANALYZING → PENDING_REVIEW → CODING → ...)
- [ ] **State Checkpointing**: Save state *before* and *after* every workflow step so that if the server crashes, the task can resume from the exact point of failure.
- [ ] Error handling — retry with different agent/model on failure
- [ ] Event emission — publish task events for dashboard (including spec review notifications)

---

# Phase 3b — Workflow Engine + Prompt Assembly + Execution UI

> **Goal:** Build the full DAG workflow engine with parallel execution, prompt assembly with rule engine integration, and the execution monitoring UI.
> **Depends on:** Phase 3a (Sandbox + Agent Manager + Orchestrator Core)

---

## Task 5: Workflow Engine (Spec-driven & Parallel Execution)

**Files:**
- Create: `server/internal/workflow/engine.go` — workflow definition & runner
- Create: `server/internal/workflow/steps.go` — individual step implementations
- Create: `server/internal/workflow/schema.go` — JSON schemas for agent I/O

**Scope:**
- [ ] **Spec-driven Development**: Define strict JSON/YAML schemas for inputs and outputs between agents. The engine MUST validate the agent's output against the schema before moving to the next step.
- [ ] Define workflow as a DAG of steps:
  1. **Analyze** — Planner agent analyzes task, classifies complexity, generates structured analysis. If Easy → auto-approve and continue. If Medium/Hard → pause for human review (via `spec_status`).
  2. **Plan** — (After spec is approved) Planner agent decomposes task into JSON sub-tasks with individual specs.
  3. **Code (Parallel)** — Spin up multiple Sandbox containers to execute independent sub-tasks concurrently (e.g., Frontend & Backend).
  4. **Merge** — Specialized Git merge step to resolve conflicts if parallel agents touched the same files.
  5. **Review** — Reviewer agent performs code review.
  6. **Fix** — Original agent fixes review feedback.
  7. **Test** — QA agent runs full test suite.
  8. **PR** — Create PR via gitops package.
- [ ] Step result passing — output of one step is strictly typed input to next.
- [ ] Auto-retry loop — on validation failure or CI failure, create fix task and re-run.

---

## Task 6: Prompt Assembly (Rule Engine Integration)

**Files:**
- Create: `server/internal/orchestrator/prompt.go`

**Scope:**
- [ ] Fetch global rules (scope=global) → inject into system prompt
- [ ] Fetch project rules (scope=project) → inject into task context
- [ ] Conflict detection — reject project rules that override global rules
- [ ] Attach: task description, relevant files, code context (placeholder for RAG)

---

## Task 7: API Endpoints for Orchestration

**Files:**
- Modify: `server/internal/handler/task.go`
- Create: `server/internal/handler/workflow.go`

**Scope:**
- [ ] `POST /api/v1/tasks/:id/execute` — trigger task execution
- [ ] `GET /api/v1/tasks/:id/logs` — stream execution logs (SSE)
- [ ] `GET /api/v1/tasks/:id/workflow` — get workflow status & step progress
- [ ] `POST /api/v1/tasks/:id/approve` — human approval to merge PR

---

## Task 8: Web UI — Execution Monitoring

**Scope:**
- [ ] Task detail page — real-time log streaming
- [ ] Workflow progress bar — step indicators (analyze/review/plan/code/review/fix/test/PR) with Easy vs Medium/Hard track indicator
- [ ] **Spec review notification** — alert reviewers when Medium/Hard tasks need spec approval
- [ ] Agent activity panel — which agent is working on what

---

## Execution Order

```
── Phase 3a ──
Task 1 → 2 → 3 → 4   (Sandbox → Secrets → Agent Manager → Orchestrator Core)

── Phase 3b (after 3a is verified) ──
Task 5 → 6 → 7 → 8   (Workflow Engine → Prompt Assembly → API → UI)
```

**Why the split?**
- Phase 3a delivers a working sandbox + basic task execution (sequential)
- Phase 3b adds the advanced DAG workflow, parallel execution, and rule engine
- Team can validate 3a before committing to 3b complexity

## Testing Requirements

| Layer | Tool | Minimum Coverage |
|-------|------|------------------|
| **Sandbox** | Integration tests | Container create/destroy, command exec, output capture |
| **Agent Manager** | Unit tests | Queue, assignment, status lifecycle |
| **Orchestrator** | Integration tests | End-to-end: task → agent → sandbox → result |
| **Workflow Engine** | Unit + integration | Step execution, DAG traversal, parallel dispatch, merge |
| **Prompt Assembly** | Unit tests | Rule injection, conflict detection |

## Verification

```bash
# Phase 3a: Create a task via API, trigger execution, monitor logs
curl -X POST localhost:8080/api/v1/tasks/{id}/execute
curl localhost:8080/api/v1/tasks/{id}/logs  # SSE stream

# Phase 3b: Run a full workflow
curl -X POST localhost:8080/api/v1/tasks/{id}/execute  # triggers DAG workflow
curl localhost:8080/api/v1/tasks/{id}/workflow  # check step progress
```
