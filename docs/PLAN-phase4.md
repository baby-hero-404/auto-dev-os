# Phase 4 Implementation Plan — AI Gateway (Tier Routing) + Skill System

> **Status:** 📋 Planned
> **Depends on:** Phase 3b (Workflow Engine + Prompt Assembly)

**Goal:** Build the intelligent LLM routing layer and a dynamic skill system so agents can use the right model for each task and leverage reusable tool capabilities.

---

## References

> Study these resources before starting implementation.

### Learning Report — `resources/Learning_Report.md`

| Section | Key Learnings for Phase 4 |
|---------|---------------------------|
| §4 9Router | **Centralized Routing** — proxy architecture, fallback models, load balancing, quota/token tracking, Open-SSE streaming |
| §9 Free Claude Code | **Tier-based Model Routing** — Opus/Sonnet/Haiku routing, protocol normalization (OpenAI→Anthropic), drop-in proxy design |
| §2 OpenClaw | **Skills & Tools** — standardized input/output definitions for AI tools, skill-based architecture |
| §7 Superpowers | **Skill Plugins System** — portable skill format (Markdown-based), reusable across agent platforms |

### Reference Doc — `resources/Reference_doc.md`

| Section | Key Learnings for Phase 4 |
|---------|---------------------------|
| §3.1 | **Modular architecture** — adapter pattern for LLM providers, easy to swap/extend |
| §3.4 | **Flexible Skill System** — JSON Schema for tool functions, Skill Registry, Progressive Disclosure |
| §3.7 | **Observability** — token usage tracking, metrics, tracing for LLM calls |

### Deep Code References in `resources/`

| Component | Path to Study | What to Learn / Reuse |
|-----------|---------------|-----------------------|
| **Tier-based Routing** | `resources/free-claude-code/providers/` & `core/` | Study how requests are intercepted, token-counted, and routed to Opus/Sonnet/Haiku based on task complexity. |
| **SSE Streaming** | `resources/9router/open-sse/` | Lightweight implementation of Server-Sent Events for streaming LLM responses. |
| **Skill Registry** | `resources/prompt_base/registry.min.json` | Adopt this exact JSON structure for the centralized Skill Registry. |
| **Workflows** | `resources/prompt_base/antigravity/global_workflows/` | Markdown-based workflow templates (e.g. `/plan`, `/brainstorm`) that agents can execute. |
| **Portable Skills** | `resources/prompt_base/antigravity/skills/` | Markdown-based skill definitions that are easy for humans to read and AI to execute. |

---

## ⚠️ Pre-requisite: Human Review Gate

> **MANDATORY:** Before starting Phase 4, the team must review:
> 1. All Phase 3a/3b deliverables are verified and tested.
> 2. Existing `server/pkg/llm/` code — understand current provider abstraction.
> 3. `resources/free-claude-code/providers/` — tier-based routing patterns.
> 4. `resources/9router/open-sse/` — SSE streaming reference.
> 5. `resources/prompt_base/registry.min.json` — Skill Registry JSON structure.
>
> **Only proceed after the team signs off.**

---

## Task 1: LLM Gateway — Tier-based Routing

**Files:**
- Modify: `server/pkg/llm/provider.go` — add tier metadata
- Create: `server/pkg/llm/router.go` — routing engine
- Create: `server/pkg/llm/fallback.go` — fallback chain

**Scope:**
- [ ] Route by task complexity: `easy` → fast/cheap model, `hard` → powerful model
- [ ] Provider routing — switch between OpenAI, Anthropic, Gemini
- [ ] Protocol normalization — unified request/response format
- [ ] Fallback chain — if primary model fails, try next provider
- [ ] **Cost Circuit Breaker**: Hard limits to automatically kill an LLM loop if the task exceeds a defined budget cap ($X or Y tokens), preventing runaway costs.

---

## Task 2: LLM Gateway — Token Tracking & Analytics

**Files:**
- Create: `server/migration/000006_token_usage.up.sql`
- Create: `server/migration/000006_token_usage.down.sql`

**Scope:**
- [ ] Track: model, tokens_in, tokens_out, cost, latency per request
- [ ] Aggregate by project, agent, time period
- [ ] API endpoint: `GET /api/v1/analytics/token-usage`

---

## Task 3: Skill System — Runtime

**Files:**
- Modify: `server/internal/orchestrator/prompt.go` — inject available skills
- Create: `server/internal/orchestrator/skill_executor.go`

**Scope:**
- [ ] Skills as tool definitions (JSON schema) passed to LLM
- [ ] Skill execution — when LLM requests a tool call, execute the skill
- [ ] Built-in skills: `run_tests`, `analyze_logs`, `generate_docs`, `create_migration`, `search_code`
- [ ] Skill result injection — feed tool output back into LLM conversation

---

## Task 4: Skill CRUD Enhancement

**Files:**
- Modify: `server/internal/handler/skill.go`

**Scope:**
- [ ] `POST /api/v1/skills/:id/test` — dry-run a skill with sample input
- [ ] `GET /api/v1/agents/:id/skills` — list skills available to an agent
- [ ] `POST /api/v1/agents/:id/skills` — assign skills to an agent

---

## Task 5: Agent Evals (LLM-as-a-Judge)

**Files:**
- Create: `server/internal/evals/evaluator.go`
- Create: `server/internal/evals/datasets.go`

**Scope:**
- [ ] Golden datasets: store sample inputs and expected behaviors for skills and prompts.
- [ ] LLM-as-a-judge: use an advanced model (e.g., Opus/GPT-4o) to grade outputs of smaller models.
- [ ] CI/CD for AI: automatically run evals when modifying the Prompt Assembly or Skill definitions.
- [ ] Reject deployment of new skills if eval scores drop below a threshold.

---

## Task 6: Web UI — Gateway Dashboard

**Scope:**
- [ ] Token usage charts (by project, by model, over time)
- [ ] Model configuration page — manage providers, API keys, tier mappings
- [ ] Skill management page — create/edit skills, test with sample inputs
- [ ] Evals Dashboard — view historical eval scores and LLM-as-a-judge reasoning

---

## Execution Order

```
Task 1 → 2 (Gateway core)
Task 3 → 4 (Skill system)
Task 5     (Agent Evals)
Task 6     (UI — can parallel after Tasks 1-5)
```

## Testing Requirements

| Layer | Tool | Minimum Coverage |
|-------|------|------------------|
| **LLM Gateway** | Unit + integration | Routing logic, fallback chain, circuit breaker, protocol normalization |
| **Token Tracking** | Unit tests | Usage recording, aggregation queries |
| **Skill System** | Integration tests | Skill execution, result injection, tool call round-trip |
| **Evals** | Integration tests | Golden dataset comparison, score threshold enforcement |
