# Phase 5 Implementation Plan — Dashboard + Analytics + PR & Human Review

> **Status:** 📋 Planned
> **Depends on:** Phase 4 (AI Gateway + Skill System)

**Goal:** Build the full observability and review layer — project dashboards, agent metrics, AI-generated PR summaries, and human approval workflows.

---

## References

> Study these resources before starting implementation.

### Learning Report — `resources/Learning_Report.md`

| Section | Key Learnings for Phase 5 |
|---------|---------------------------|
| §3 AI-SDLC | **Operator TUI/Dashboard** — live pipeline status, PR tracking, dependency graphs, analytics |
| §4 9Router | **Open-SSE** — streaming technique for real-time log and event delivery to dashboard |

### Reference Doc — `resources/Reference_doc.md`

| Section | Key Learnings for Phase 5 |
|---------|---------------------------|
| §2.1 AI-SDLC | **Cross-Harness Review** — attestation envelopes, quality gates for PR validation |
| §3.5 | **CI/CD Feedback Loop** — auto-create fix tasks on CI failure, webhook-triggered workflows |
| §3.7 | **Observability & Debugging** — agent tracing, performance metrics, token usage dashboards |
| §3.8 | **HITL Design** — clear approval points, developer-friendly review interface |

### Deep Code & UI References in `resources/`

| Component | Path to Study | What to Learn / Reuse |
|-----------|---------------|-----------------------|
| **Operator TUI / Dashboard** | `resources/ai-sdlc/dashboard/` | Check how live pipeline status and PR tracking are visualized for human operators. |
| **Open-SSE (Streaming)** | `resources/9router/open-sse/` | Lightweight Server-Sent Events architecture for streaming real-time logs to the UI. |
| **Stats Cards (UI)** | `resources/ui-demo/components/dashboard/stats-cards.tsx` | Dashboard overview stat cards for the Next.js UI. |
| **Metrics Chart (UI)** | `resources/ui-demo/components/dashboard/metrics-chart.tsx` | Recharts-based data visualization for agent analytics. |
| **Workflow Timeline (UI)** | `resources/ui-demo/components/dashboard/workflow-timeline.tsx` | Step-by-step workflow visualization for the Orchestrator pipeline. |

---

## ⚠️ Pre-requisite: Human Review Gate

> **MANDATORY:** Before starting Phase 5, the team must review:
> 1. All Phase 4 deliverables are verified and tested.
> 2. Token tracking data is flowing correctly.
> 3. `resources/ai-sdlc/dashboard/` — Dashboard UX patterns.
> 4. `resources/ui-demo/components/dashboard/` — Stats cards, metrics chart, workflow timeline components.
>
> **Only proceed after the team signs off.**

---

## Task 1: Dashboard Analytics Backend

**Files:**
- Create: `server/internal/service/analytics.go`
- Create: `server/internal/handler/analytics.go`

**Scope:**
- [ ] `GET /api/v1/analytics/overview` — total tasks, agents, success rate, avg completion time
- [ ] `GET /api/v1/analytics/agents` — per-agent: success rate, retry count, token usage, task count
- [ ] `GET /api/v1/analytics/tasks` — task throughput over time, status distribution
- [ ] `GET /api/v1/analytics/workflows` — workflow completion rates, average step durations

---

## Task 2: Audit Logs & Observability Tracing

**Files:**
- Create: `server/migration/000007_audit_logs.up.sql`
- Create: `server/migration/000007_audit_logs.down.sql`

**Scope:**
- [ ] Record immutable Audit Logs for all critical agent actions (file edits, API calls, tool executions).
- [ ] Implement request tracing (OpenTelemetry/Jaeger concept) to track the lifecycle of a task across Orchestrator, Gateway, and Sandbox.
- [ ] Expose `GET /api/v1/audit/logs` for compliance and debugging.

---

## Task 3: PR Generation & Review System

**Files:**
- Modify: `server/internal/gitops/github.go` — enrich PR creation
- Create: `server/internal/orchestrator/pr_generator.go`

**Scope:**
- [ ] Auto-generate PR body: title, summary, changed files list, risk assessment
- [ ] AI PR assistant — when reviewer asks a question on the PR, agent explains context
- [ ] Merge policy enforcement — require: all tests pass + human approval
- [ ] `POST /api/v1/tasks/:id/pr/approve` — human approves PR from platform
- [ ] `POST /api/v1/tasks/:id/pr/reject` — human rejects with feedback → triggers fix cycle

---

## Task 4: Web UI — Analytics Dashboard

**Scope:**
- [ ] Project dashboard — task status distribution chart, active agents, open PRs
- [ ] Agent metrics panel — success rate, retries, token consumption, avg task time
- [ ] Workflow timeline — visual step-by-step execution history
- [ ] Token cost tracker — burn rate, budget remaining

---

## Task 5: Web UI — PR Review & Audit Interface

**Scope:**
- [ ] PR detail page — diff viewer, AI-generated summary, risk assessment badge
- [ ] Inline comments — reviewer can ask questions → AI responds
- [ ] Approve/Reject buttons — triggers merge or fix cycle
- [ ] Audit Log Viewer — table showing all agent system actions and API calls for compliance

---

## Execution Order

```
Task 1 → 2 → 4 (Analytics & Audit)
Task 3 → 5 (PR system & Review UI)
```

## Testing Requirements

| Layer | Tool | Minimum Coverage |
|-------|------|------------------|
| **Analytics** | Integration tests | Aggregation queries, time-series data correctness |
| **Audit Logs** | Unit + integration | Immutable log recording, query filtering |
| **PR Generation** | Integration tests | PR body generation, merge policy enforcement |
| **Web UI** | Playwright E2E | Dashboard renders, PR review flow, audit log viewer |
