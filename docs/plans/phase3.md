# Phase 3 Implementation Plan — Orchestrator + Agent Manager + Sandbox + Workflow Engine

> **Status:** 🕒 Phase 3a: ✅ COMPLETED | Phase 3b: ✅ COMPLETED
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
- [x] Create & destroy Docker containers per task
- [x] Mount git worktree (cloned repo) as volume
- [x] Execute commands inside container (git, tests, build)
- [x] Capture stdout/stderr output and stream to API
- [x] Resource limits (CPU, memory, timeout)
- [x] **Network Isolation**: Disable default internet access. Route traffic through an internal proxy to monitor/block unauthorized outbound connections.
- [x] **Dual-Container Execution**: Implement a pre-build step in a separate container WITH network access (but no secrets injected) to resolve and cache dependencies (e.g. `npm install`, `go mod download`). Run the actual code editing, compile, and unit-test steps in a container with `network: "none"` and secrets injected.
- [x] **Secretless Tooling**: Avoid injecting secrets during local edit/compile/test phases. Inject secrets only during deployment or staging integration test phases.
- [x] **Policy Engine**: Enforce declarative policies (e.g., block `rm -rf`, restrict write access to specific directories) by intercepting shell commands before execution.

---

## Task 2: Secret Vault Management

**Files:**
- Create: `server/migration/000005_secrets_and_agents.up.sql` — AES encrypted storage + Org-scoped agent refactor (with assignment strategy support)
- Create: `server/internal/service/secrets.go`

**Scope:**
- [x] Store project-specific environment variables securely (AES-GCM encryption).
- [x] Retrieve and decrypt secrets at runtime.
- [x] Inject secrets directly into the Sandbox container as secure ENV vars.
- [x] Prevent secrets from appearing in agent logs or SSE streams.

---

## Task 3: Agent Manager & Job Queue

**Files:**
- Create: `server/internal/orchestrator/agent_manager.go`
- Create: `server/internal/orchestrator/queue.go`
- Modify: `server/pkg/models/agent.go` — add OrgID, `assignment_strategy` field (`AUTO_JOIN`/`MANUAL`), and remove project association constraints
- Modify: `server/internal/repository/agent.go` — update agent CRUD to query Org level & Join table, and resolve auto-joined agents
- Modify: `server/internal/service/agent.go` — add "hire" (Org level, handling auto-join logic) and "assign" (Project level manual mapping) logic
- Modify: `server/internal/handler/agent.go` — update routes and handlers (REST API: `POST /api/v1/organizations/:id/agents` and `POST /api/v1/projects/:id/agents` for assignment)

**Scope:**
- [x] **Job Queue**: Implement PostgreSQL `SELECT ... FOR UPDATE SKIP LOCKED` for robust background task queuing.
- [x] **Org-scoped Staff Pool**: Hire/create agents at the Organization level, assigning them to multiple projects via the many-to-many `project_agents` join table.
- [x] **Assignment Strategy**: Implement `assignment_strategy` for agents (Auto-Join vs Manual). Auto-joined agents are automatically available to all projects within the organization, while manually added agents require mapping in `project_agents`.
- [x] Agent pool — track idle/busy agents in the database
- [x] Agent assignment — match task complexity + role to agent capabilities (loading model/provider dynamically from GORM database, while prompt templates are filesystem-based)
- [x] Concurrent agent execution — goroutine per agent task, pulling from queue.
- [x] Agent status lifecycle: `idle` → `assigned` → `running` → `idle`

---

## Task 4: Orchestrator Core & Checkpointing

**Files:**
- Create: `server/internal/orchestrator/orchestrator.go`

> **Aligns with:** `docs/manual/Roadmap.md` §2 — Complexity-based branching (Easy/Medium/Hard)

**Scope:**
- [x] Receive task from API → trigger analysis by Planner agent
- [x] **Complexity-based branching** (implements Roadmap §2):
  - [x] **Easy tasks** (`spec_status = AUTO_APPROVED`): Skip human review → assign agent → dispatch to sandbox immediately.
  - [x] **Medium/Hard tasks** (`spec_status = PENDING_REVIEW`): Pause execution → wait for human approval (`spec_status = APPROVED`) before dispatching to sandbox.
  - [x] **Clarification loop**: If agent needs more info, set `spec_status = CHANGES_REQUESTED`, emit clarification questions via SSE, and wait for developer response before re-analyzing.
- [x] **High-level Task State Machine**: Simplify the high-level task database statuses to `TODO`, `IN_PROGRESS`, `FAILED`, and `COMPLETED`.
- [x] **DAG Step Checkpointing**: Delegate fine-grained step execution states (e.g., Step `Analyze` -> `SUCCESS`, Step `Code_Backend` -> `RUNNING`) to the Workflow Engine. Save state checkpoints (DB/JSON payload) *before* and *after* each individual step execution in the DAG, allowing resumption of parallel execution paths if the server restarts.
- [x] Error handling — retry with different agent/model on failure
- [x] Event emission — publish task events for dashboard (including spec review notifications)

---

# Phase 3b — Workflow Engine + Prompt Assembly + Execution UI

> **Goal:** Build the full DAG workflow engine with parallel execution, prompt assembly with rule engine integration, and the execution monitoring UI.
> **Depends on:** Phase 3a (Sandbox + Agent Manager + Orchestrator Core)

### Design Decisions (Finalized)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **DAG Architecture** | **Option C: Hybrid Step Registry** | Steps are compiled Go code (type-safe, debuggable). DAG graph is a Go struct default, upgradable to DB/JSON later. |
| **Parallel Workspace** | **Shared volume + mutex** | Single git worktree shared across parallel containers. File-level mutex coordination prevents conflicts. |
| **Monitoring UI** | **Dedicated page** `/projects/[id]/tasks/[taskID]/monitor` | Separation of concerns; real-time SSE streaming with step progress bar. |
| **ContextRetriever** | **4-tier priority stub** | Explicit files → Import-neighbor scan → Keyword fallback → Token budget cap. No full-project reads. |

---

## Task 5: Workflow Engine (Hybrid Step Registry DAG)

**Files:**
- Create: `server/internal/workflow/engine.go` — DAG runner with Step Registry
- Create: `server/internal/workflow/step.go` — `Step` interface + concrete step implementations
- Create: `server/internal/workflow/graph.go` — DAG graph definition (default workflow)
- Create: `server/internal/workflow/schema.go` — JSON schemas for step I/O validation

**Architecture: Hybrid Step Registry**

Step implementations are compiled Go structs implementing a common `Step` interface. The DAG graph linking steps together is a simple Go struct (upgradable to DB/JSON config in future phases).

```go
// Step is the unit of work in a workflow DAG.
type Step interface {
    Name() string
    Run(ctx context.Context, input StepIO) (StepIO, error)
}

// StepIO is the typed data envelope passed between steps.
type StepIO struct {
    TaskID    string
    JobID     string
    Data      json.RawMessage // step-specific payload
    Files     []string        // affected file paths
}

// DAGNode links a step to its dependencies and parallelism.
type DAGNode struct {
    Step         Step
    DependsOn    []string // names of prerequisite steps
    Parallel     bool     // if true, runs concurrently with siblings
}
```

**Default Workflow DAG:**
```
Analyze ──→ Plan ──→ Code(parallel) ──→ Merge ──→ Review ──→ Fix ──→ Test ──→ PR
```

**Scope:**
- [x] **Step Interface & Registry**: Define `Step` interface with `Name()` and `Run()`. Register 8 concrete steps (`StepAnalyze`, `StepPlan`, `StepCode`, `StepMerge`, `StepReview`, `StepFix`, `StepTest`, `StepPR`).
- [x] **DAG Graph Builder**: Build the default workflow graph as a Go struct with `DAGNode` entries. Validate graph for cycles (topological sort) at startup.
- [x] **DAG Runner**: Execute steps respecting dependency order. Sequential steps run one-at-a-time. Parallel-flagged siblings (e.g. `StepCode` sub-tasks) run concurrently via goroutines.
- [x] **Step I/O Validation**: Define JSON schemas per step. Validate output of step N against expected input schema of step N+1 before proceeding.
- [x] **Parallel Code Execution**: For the `Code` step, decompose sub-tasks and spin up multiple Sandbox containers concurrently. **Shared workspace volume with file-level mutex** coordination to prevent write conflicts.
- [x] **Merge Step**: After parallel code execution, run `git merge` / `git diff` to detect and resolve conflicts. If unresolvable, escalate to a fix step.
- [x] **Auto-retry Loop**: On step failure or schema validation error, retry up to `maxRetries` (configurable per step). On CI failure, create a fix sub-task and re-enter the Fix→Test loop.
- [x] **Checkpoint Integration**: Save `workflow_checkpoints` before and after each step. On server restart, resume from the last successful checkpoint.

---

## Task 6: Prompt Assembly (Rule Engine & Token Optimization Integration)

**Files:**
- Create: `server/internal/orchestrator/prompt.go` — prompt builder & rule injection
- Create: `server/internal/orchestrator/context.go` — `ContextRetriever` interface + 4-tier stub

**Architecture: 4-Tier Priority ContextRetriever Stub**

```go
type ContextRetriever interface {
    Retrieve(ctx context.Context, task models.Task, repoPath string) ([]ContextFile, error)
}

type ContextFile struct {
    Path     string
    Content  string
    Truncated bool
}
```

**Retrieval Priority Order:**
1. **Tier 1 — Explicit Files**: If the task analysis declares `affected_files`, read those files directly. This is the most reliable context source.
2. **Tier 2 — Import-Neighbor Scan**: For each explicit file, parse import statements (simple regex, no full AST) and include neighboring files from the imported packages (e.g. `pkg/llm/*.go`).
3. **Tier 3 — Keyword/Path Fallback**: If no explicit files exist, extract keywords from the task title+description and search file paths matching those keywords (e.g. task mentioning "sandbox" finds `internal/sandbox/*.go`).
4. **Tier 4 — Token Budget Cap**: Hard limits to prevent context explosion:
   - `MaxFiles: 8`
   - `MaxBytesPerFile: 20_000`
   - `MaxTotalBytes: 80_000`
   - Files exceeding limits are truncated with `// ... truncated ...` markers.

**Scope:**
- [x] Load system prompt templates dynamically from localized Git-tracked Markdown files (e.g. `resources/prompt_base/antigravity/agents/*.md` based on Agent Role)
- [x] Fetch global rules (scope=global) from DB → inject into system prompt
- [x] Fetch project rules (scope=project) from DB → inject into task context
- [x] Conflict detection — reject project rules that override global rules
- [x] Attach: task description, relevant files, code context via `ContextRetriever`
- [x] **ContextRetriever Stub**: Implement 4-tier priority retrieval (explicit files → import scan → keyword fallback → token budget). Drop-in replaceable with Phase 6's pgvector RAG.
- [x] **Prompt Caching Layout**: Structure the prompt assembly output to place static elements (System Prompt, Roles, Global/Project Rules) at the beginning of the payload to maximize LLM Prompt Caching hit rates.
- [x] **Conversation Truncation & Summarization**: Implement history sliding-window and summarization for long-running agent execution loops to prevent token bloat.
- [x] **Context Pruning**: Extract snippet-level code context (imports, function signatures) rather than full source files. Apply `MaxBytesPerFile` truncation with markers.


---

## Task 7: API Endpoints for Orchestration

**Files:**
- Modify: `server/internal/handler/task.go`
- Create: `server/internal/handler/workflow.go`

**Scope:**
- [x] `POST /api/v1/tasks/:id/execute` — trigger task execution
- [x] `GET /api/v1/tasks/:id/logs` — stream execution logs (SSE)
- [x] `GET /api/v1/tasks/:id/workflow` — get workflow status & step progress
- [x] `POST /api/v1/tasks/:id/approve` — human approval to merge PR

---

## Task 8: Web UI — Execution Monitoring (Dedicated Page)

**Files:**
- Create: `web/src/app/projects/[id]/tasks/[taskID]/monitor/page.tsx` — dedicated monitoring page
- Create: `web/src/components/workflow/workflow-progress.tsx` — DAG step progress bar
- Create: `web/src/components/workflow/log-stream.tsx` — real-time SSE log viewer
- Create: `web/src/components/workflow/agent-panel.tsx` — agent activity panel
- Modify: `web/src/lib/api.ts` — add `executeTask()`, `getWorkflowStatus()` client methods
- Modify: `web/src/lib/types.ts` — add `WorkflowStatus`, `WorkflowJob`, `TaskLog` types

**Route:** `/projects/[id]/tasks/[taskID]/monitor`

**Scope:**
- [x] **Dedicated Monitor Page**: Full-screen task execution monitoring at `/projects/[id]/tasks/[taskID]/monitor`. Shows real-time workflow state, logs, and agent info.
- [x] **Workflow Progress Bar**: Visual DAG step indicators (`Analyze → Plan → Code → Merge → Review → Fix → Test → PR`) with color-coded status (pending/running/done/failed). Show Easy vs Medium/Hard track indicator.
- [x] **Real-time Log Stream**: SSE-powered scrolling log viewer consuming `GET /tasks/:id/logs?stream=true`. Auto-scroll with pause/resume.
- [x] **Spec Review Notification**: Alert banner when Medium/Hard tasks are awaiting spec approval. Inline approve/reject buttons.
- [x] **Agent Activity Panel**: Show which agent is assigned, its status, model/provider info, and current step.
- [x] **Execute Button**: Trigger task execution from UI via `POST /tasks/:id/execute`. Link from project detail page task cards.

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
