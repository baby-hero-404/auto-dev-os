# Phase 6 Implementation Plan — Remote Chatbots + Episodic Memory + Self-improving Agents

> **Status:** 📋 Planned
> **Depends on:** Phase 5 (Dashboard + PR Review)

**Goal:** Enable multi-channel interaction (Discord/Telegram/Slack), build episodic memory for long-term agent learning, and implement self-improving agent capabilities.

---

## References

> Study these resources before starting implementation.

### Learning Report — `resources/Learning_Report.md`

| Section | Key Learnings for Phase 6 |
|---------|---------------------------|
| §6 AgentMemory | **Episodic Memory** — long-term storage of architectural decisions, RAG with vector search |
| §8 Hermes Agent | **Self-improving Learning Loop** — auto-create and refine skills from experience, cross-session recall (FTS5), user profiling |
| §8 Hermes Agent | **Multi-backend Execution** — Local, Docker, Modal, Vercel Sandbox; multi-channel chat (Telegram, Discord, CLI) |
| §9 Free Claude Code | **Remote Coding Sessions** — Discord and Telegram bot integration for controlling coding agents remotely |
| §2 OpenClaw | **Multi-channel Inbox** — plugin architecture for WhatsApp, Telegram, Slack, Discord, Google Chat, Signal, iMessage |

### Reference Doc — `resources/Reference_doc.md`

| Section | Key Learnings for Phase 6 |
|---------|---------------------------|
| §2.2 OpenClaw | **Multi-channel Integration** — local-first gateway as control plane, multi-agent routing per channel |
| §2.2 OpenClaw | **Voice Wake + Talk Mode** — voice activation and continuous conversation |
| §3.3 | **Knowledge Base** — centralized store for docs, RFCs, architecture notes; RAG for context injection |
| §3.6 | **Security & Governance** — RBAC, audit logs, policy engine, secret management |

### Deep Code References in `resources/`

| Component | Path to Study | What to Learn / Reuse |
|-----------|---------------|-----------------------|
| **Chatbot Platforms** | `resources/free-claude-code/messaging/platforms/` | Reference `discord.py` and `telegram.py` for connecting bots to agent execution loops. |
| **Multi-channel Plugins** | `resources/openclaw/extensions/` | Study the extension pattern for adding new messaging platforms (Slack, WhatsApp). |
| **Episodic Memory / MCP** | `resources/agentmemory/packages/mcp/` | Model Context Protocol implementation for giving agents access to long-term memory. |
| **Self-improving Loop** | `resources/hermes-agent/hermes_state.py` & `run_agent.py` | Examine the core state loop where agents reflect on failures and generate new routines. |

---

## ⚠️ Pre-requisite: Human Review Gate

> **MANDATORY:** Before starting Phase 6, the team must review:
> 1. All Phase 5 deliverables are verified and tested.
> 2. `resources/free-claude-code/messaging/platforms/` — Discord/Telegram bot patterns.
> 3. `resources/agentmemory/` — Episodic memory and vector search.
> 4. `resources/hermes-agent/` — Self-improving agent loop.
>
> **Only proceed after the team signs off.**

---

## Task 1: Multi-channel Chatbot Gateway

**Files:**
- Create: `server/internal/chatbot/gateway.go` — unified message interface
- Create: `server/internal/chatbot/discord.go`
- Create: `server/internal/chatbot/telegram.go`
- Create: `server/internal/chatbot/slack.go`

**Scope:**
- [ ] Unified message format — normalize incoming messages from all platforms
- [ ] Command parsing — `/task create`, `/task status`, `/pr approve`, `/agent status`
- [ ] Progress streaming — push task updates to chat channels
- [ ] Human-in-the-loop via chat — approve/reject PRs with chat commands
- [ ] Voice note transcription — audio → text → task creation (Whisper API)

---

## Task 2: Episodic Memory (pgvector)

**Files:**
- Create: `server/migration/000008_episodic_memory.up.sql` — enhance memories table
- Create: `server/migration/000008_episodic_memory.down.sql`
- Create: `server/internal/handler/memory.go`

**Scope:**
- [ ] Store agent experiences as embeddings (task context + outcome + learnings)
- [ ] Semantic search — find relevant past experiences for new tasks
- [ ] User modeling — track developer preferences and patterns
- [ ] RAG pipeline — inject relevant memories into agent prompts
- [ ] Memory decay — reduce weight of old/irrelevant memories over time

---

## Task 3: Self-improving Agent Loop

**Files:**
- Modify: `server/internal/orchestrator/orchestrator.go`
- Create: `server/internal/orchestrator/learning.go`

**Scope:**
- [ ] Post-task evaluation — agent reflects on outcome (success/failure/retries)
- [ ] Skill generation — agent proposes new skills based on recurring patterns
- [ ] Performance tracking — historical success rates feed into agent selection
- [ ] Confidence scoring — agent rates its own confidence before execution

---

## Task 4: Web UI — Memory & Learning Dashboard

**Scope:**
- [ ] Agent memory browser — search and explore stored memories
- [ ] Learning history — show how agent capabilities evolved over time
- [ ] Chatbot configuration — connect/disconnect channels, view message history

---

## Execution Order

```
Task 1 (Chatbot — independent)
Task 2 → 3 (Memory → Learning loop)
Task 4 (UI — after Tasks 1-3)
```

## Testing Requirements

| Layer | Tool | Minimum Coverage |
|-------|------|------------------|
| **Chatbot Gateway** | Integration tests | Message normalization, command parsing, platform adapters |
| **Episodic Memory** | Unit + integration | Embedding storage, semantic search accuracy, memory decay |
| **Self-improving Loop** | Unit tests | Evaluation logic, skill generation, confidence scoring |
| **Web UI** | Playwright E2E | Memory browser, chatbot config, learning history |
