# 📚 Reference Projects Analysis

> **Purpose**: Extract actionable learnings from 20 reference projects to improve Auto Code OS.
> **Generated**: 2026-07-17 | **Focus**: What can we adopt to make our AI-native SDLC platform best-in-class.

## Reference Index

| # | Project | Category | Relevance | Report |
|---|---------|----------|-----------|--------|
| 1 | Multica | Agent Platform | ⭐⭐⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-multica.md) |
| 2 | AI-SDLC | SDLC Framework | ⭐⭐⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-ai-sdlc.md) |
| 3 | Hermes Agent | AI Agent | ⭐⭐⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-hermes-agent.md) |
| 4 | AgentMemory | Agent Memory | ⭐⭐⭐⭐⭐ | [Report](./memory/DISCOVERY-agentmemory.md) |
| 5 | Aider | AI Pair Programming | ⭐⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-aider.md) |
| 6 | OpenSpec | Spec-Driven Dev | ⭐⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-openspec.md) |
| 7 | 9Router | LLM Router | ⭐⭐⭐⭐ | [Report](./infrastructure/DISCOVERY-9router.md) |
| 8 | Zep | Memory + Knowledge | ⭐⭐⭐⭐ | [Report](./memory/DISCOVERY-zep.md) |
| 9 | Headroom | Token Compression | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-headroom.md) |
| 10 | Claw Compactor | Token Compression | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-claw-compactor.md) |
| 11 | RTK | Token Compression | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-rtk.md) |
| 12 | Caveman | Output Reduction | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-caveman.md) |
| 13 | TOON | Token Format | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-toon.md) |
| 14 | LLMLingua | Prompt Compression | ⭐⭐⭐ | [Report](./token-compression/DISCOVERY-llmlingua.md) |
| 15 | OpenClaw | AI Gateway | ⭐⭐⭐ | [Report](./infrastructure/DISCOVERY-openclaw.md) |
| 16 | Free Claude Code | Proxy | ⭐⭐ | [Report](./infrastructure/DISCOVERY-free-claude-code.md) |
| 17 | Superpowers | Skills Framework | ⭐⭐⭐ | [Report](./agent-platform/DISCOVERY-superpowers.md) |
| 18 | Prompt Base | Skills Framework | ⭐⭐ | [Report](./infrastructure/DISCOVERY-prompt-base.md) |
| 19 | Antigravity Skills | Skills Registry | ⭐⭐ | [Report](./infrastructure/DISCOVERY-antigravity-awesome-skills.md) |
| 20 | LLM Key Manager | Key Management | ⭐⭐⭐ | [Report](./infrastructure/DISCOVERY-llm-key-manager.md) |

---

## 🏆 Top 10 Applied Takeaways (Ranked by Priority)

> These are the most impactful ideas from ALL references, mapped onto Auto Code OS.

### 1. **Agent Persistent Memory System** (from agentmemory + zep)
- **What**: Hybrid search (FTS5 + vector/embedding) over session transcripts with confidence-scored knowledge items, auto-crystallization, and temporal decay.
- **Apply to Auto Code OS**: Add an `agent_memory` PostgreSQL table with `tsvector` full-text search. Store learnings from each task run (what worked, what failed, common patterns in this repo). Let the orchestrator query past task knowledge before starting new tasks.
- **Impact**: HIGH · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 3-5 days

### 2. **Definition-of-Ready Gate** (from ai-sdlc)
- **What**: A DoR gate refuses to dispatch tasks that haven't been fully specified. Operators resolve open questions before the agent starts, preventing waste.
- **Apply to Auto Code OS**: Add a `dor_check` step between `spec_review` and `coding` in the orchestrator DAG. Validate that all required fields (acceptance criteria, test expectations, file scope) are populated before dispatching to the sandbox.
- **Impact**: HIGH · **Effort**: LOW · **Risk**: LOW · **Est**: 1-2 days

### 3. **Cross-Harness Review** (from ai-sdlc)
- **What**: Different LLM providers review each other's work (Claude reviews Codex output, and vice versa). DSSE envelopes prevent "self-review" by construction.
- **Apply to Auto Code OS**: In the `reviewing` step, use a different LLM provider than the one used for `coding`. Add a `review_harness` field to the task model. The unified LLM gateway (`pkg/llm`) already supports multiple providers — wire it to enforce cross-provider review.
- **Impact**: HIGH · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 2-3 days

### 4. **Reusable Skills System** (from multica + hermes-agent + superpowers)
- **What**: Agents create skills from experience. After completing a complex task, the solution is abstracted into a reusable skill that future tasks can reference. Skills self-improve during use.
- **Apply to Auto Code OS**: Add a `skills` table and `orchestrator/skills/` module (already exists as a dir!). After a task reaches `merged`, extract patterns (prompts that worked, tool sequences, test strategies) into a skill record. In `context_loading`, query relevant skills to bootstrap the agent.
- **Impact**: HIGH · **Effort**: HIGH · **Risk**: MEDIUM · **Est**: 5-8 days

### 5. **LLM Token Compression Layer** (from headroom + claw-compactor + rtk)
- **What**: A proxy/middleware that compresses tool outputs, logs, and context before they reach the LLM. 50-90% reduction in token cost with CCR (Cached Compressible Retrieval).
- **Apply to Auto Code OS**: Integrate `headroom` as a proxy layer in the sandbox agent's LLM calls. Add compression middleware in `pkg/llm/` that strips redundant file content, deduplicates context, and summarizes verbose tool outputs before sending to the model.
- **Impact**: HIGH · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 2-3 days

### 6. **Worktree-Based Task Isolation** (from ai-sdlc)
- **What**: Each task runs in an isolated `git worktree` instead of branch switching. Parent worktree is read-only. Multiple tasks can execute in parallel without conflicts.
- **Apply to Auto Code OS**: Replace the current sandbox branch-checkout model with `git worktree add .worktrees/<task-id>` in `orchestrator/gitops/`. This enables true parallel task execution and prevents half-completed work from polluting the main workspace.
- **Impact**: HIGH · **Effort**: MEDIUM · **Risk**: MEDIUM · **Est**: 3-4 days

### 7. **Real-Time WebSocket Execution Streaming** (from multica)
- **What**: Full lifecycle events (enqueue → claim → start → progress → complete/fail) streamed to the frontend via WebSocket with real-time progress updates.
- **Apply to Auto Code OS**: Enhance the existing log streaming. Add structured WebSocket events for each DAG step transition. The frontend can show a live execution timeline instead of polling for status updates.
- **Impact**: MEDIUM · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 2-3 days

### 8. **Smart LLM Router with Cost Optimization** (from 9router)
- **What**: Route LLM requests to the cheapest capable model. Easy tasks → small models, complex tasks → large models. Token savings tracking dashboard.
- **Apply to Auto Code OS**: Add complexity-based routing in `pkg/llm/`. Easy tasks use cheaper models for `context_loading` and `analyzing`, while `coding` uses the premium model. Add a `token_usage` table to track costs per task.
- **Impact**: MEDIUM · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 3-4 days

### 9. **Declarative Governance Schemas** (from ai-sdlc)
- **What**: JSON Schema-defined resources for pipelines, quality gates, agent roles, and autonomy policies. Governance is configuration, not code.
- **Apply to Auto Code OS**: Define JSON schemas for task types, pipeline configurations, and quality thresholds under `docs/schemas/`. Allow users to configure the DAG flow per-project without code changes (e.g., skip `spec_review` for hotfixes).
- **Impact**: MEDIUM · **Effort**: HIGH · **Risk**: LOW · **Est**: 5-7 days

### 10. **Attestation & Audit Trail** (from ai-sdlc)
- **What**: Every code change carries a signed attestation envelope (DSSE) identifying who wrote it, who reviewed it, and which harness was used. Full audit trail.
- **Apply to Auto Code OS**: Add attestation metadata to PRs created by the orchestrator. Store agent identity, model used, prompt hash, and review chain in the task record. Critical for enterprise adoption and compliance.
- **Impact**: MEDIUM · **Effort**: MEDIUM · **Risk**: LOW · **Est**: 3-4 days

---

## 📊 Cross-Project Comparison

### Architecture & Patterns

| Project | Language | Architecture | State Management | Testing | DX |
|---------|----------|-------------|-----------------|---------|-----|
| AI-SDLC | TypeScript | Pipeline steps (0-13) | DAG + filter chain | ⭐⭐⭐⭐⭐ hermetic | ⭐⭐⭐ high config |
| Multica | TypeScript | Turborepo monorepo | Convex reactive DB | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Hermes Agent | TypeScript | Event-driven agent | Learning loop + skills DB | ⭐⭐⭐ | ⭐⭐⭐ |
| Aider | Python | CLI + repo map | Git diff tracking | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| OpenSpec | TypeScript | CLI + artifact graph | File-based spec sets | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Superpowers | TypeScript | Skill composition | Subagent delegation | ⭐⭐⭐ | ⭐⭐⭐ |
| AgentMemory | Python | Library | SQLite + hybrid search | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Zep | Go | Microservice | PostgreSQL + Neo4j | ⭐⭐⭐⭐ | ⭐⭐⭐ |
| Headroom | Rust | Proxy | CCR compression | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 9Router | Go | Reverse proxy | Cost-based routing | ⭐⭐⭐ | ⭐⭐⭐ |

### Token Cost Strategy

| Project | Approach | Claimed Savings | Integration Effort |
|---------|----------|----------------|--------------------|
| Headroom | Proxy + CCR compression | 50-90% | LOW (drop-in proxy) |
| Claw Compactor | 14-stage fusion pipeline | 60-80% | MEDIUM (Python lib) |
| RTK | Rust CLI preprocessing | 40-60% | LOW (CLI wrapper) |
| Caveman | Output shaping skill | 65% fewer output tokens | LOW (prompt skill) |
| TOON | Compact notation format | 30-50% | LOW (format spec) |
| LLMLingua | Research-grade compression | 2-10× | HIGH (model dependency) |

### Memory & Learning Strategy

| Project | Storage | Search | Knowledge Extraction |
|---------|---------|--------|---------------------|
| AgentMemory | SQLite (FTS5) | Hybrid (FTS + vector) | Auto-crystallization |
| Zep | PostgreSQL + Neo4j | Knowledge graph traversal | Entity extraction + temporal |
| Hermes Agent | File-based | Pattern matching | Skill creation from experience |

---

## Category Reports

### Agent Platform
- [DISCOVERY-multica.md](./agent-platform/DISCOVERY-multica.md) — Agent-as-teammate platform (highest relevance)
- [DISCOVERY-ai-sdlc.md](./agent-platform/DISCOVERY-ai-sdlc.md) — Spec-driven SDLC framework
- [DISCOVERY-hermes-agent.md](./agent-platform/DISCOVERY-hermes-agent.md) — Self-improving AI agent
- [DISCOVERY-aider.md](./agent-platform/DISCOVERY-aider.md) — AI pair programming (Python)
- [DISCOVERY-openspec.md](./agent-platform/DISCOVERY-openspec.md) — Spec-driven development CLI
- [DISCOVERY-superpowers.md](./agent-platform/DISCOVERY-superpowers.md) — Composable skills framework

### Memory & Knowledge
- [DISCOVERY-agentmemory.md](./memory/DISCOVERY-agentmemory.md) — Persistent agent memory
- [DISCOVERY-zep.md](./memory/DISCOVERY-zep.md) — Knowledge graphs & memory

### Token Compression
- [DISCOVERY-headroom.md](./token-compression/DISCOVERY-headroom.md) — Rust proxy, CCR compression
- [DISCOVERY-claw-compactor.md](./token-compression/DISCOVERY-claw-compactor.md) — 14-stage fusion pipeline
- [DISCOVERY-rtk.md](./token-compression/DISCOVERY-rtk.md) — Rust CLI, 100+ commands
- [DISCOVERY-caveman.md](./token-compression/DISCOVERY-caveman.md) — Output token reduction
- [DISCOVERY-toon.md](./token-compression/DISCOVERY-toon.md) — Token-Oriented Object Notation
- [DISCOVERY-llmlingua.md](./token-compression/DISCOVERY-llmlingua.md) — Microsoft prompt compression

### Infrastructure
- [DISCOVERY-9router.md](./infrastructure/DISCOVERY-9router.md) — LLM routing & token saving
- [DISCOVERY-openclaw.md](./infrastructure/DISCOVERY-openclaw.md) — Multi-channel AI gateway
- [DISCOVERY-free-claude-code.md](./infrastructure/DISCOVERY-free-claude-code.md) — Local proxy
- [DISCOVERY-llm-key-manager.md](./infrastructure/DISCOVERY-llm-key-manager.md) — Hybrid AI gateway
- [DISCOVERY-prompt-base.md](./infrastructure/DISCOVERY-prompt-base.md) — Modular AI framework
- [DISCOVERY-antigravity-awesome-skills.md](./infrastructure/DISCOVERY-antigravity-awesome-skills.md) — 1900+ skills registry
