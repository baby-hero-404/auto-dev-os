# Tasks: Phân Tích Reference Projects

## Phase 0: Chuẩn Bị & Cleanup
> Priority: P0 — Chạy trước tất cả

- [x] **T-000**: Clean old reports — xóa **tất cả** file `.md` cũ ở root `docs/references/` trừ `README.md` ✅
- [x] **T-001**: Tạo cấu trúc subfolder: `agent-platform/`, `memory/`, `token-compression/`, `infrastructure/` ✅
- [x] **T-002**: Script `scripts/analyze_references.sh` — liệt kê & phân loại 20 repo theo 4 category ✅

## Phase 1: Agent Platform (6 projects)
> Priority: P0 — Relevance cao nhất với Auto Code OS

- [x] **T-100**: `DISCOVERY-multica.md` — Multica (managed agents platform, Turborepo + Convex) ✅
- [x] **T-101**: `DISCOVERY-ai-sdlc.md` — AI-SDLC Framework (spec-driven pipeline, DoR gate, cross-harness review) ✅
- [x] **T-102**: `DISCOVERY-hermes-agent.md` — Hermes Agent (self-improving agent, learning loop, skill creation) ✅
- [x] **T-103**: `DISCOVERY-aider.md` — Aider (AI pair programming, Python, repo map, edit formats) ✅
- [x] **T-104**: `DISCOVERY-openspec.md` — OpenSpec (spec-driven dev CLI, artifact graph, validation) ✅
- [x] **T-105**: `DISCOVERY-superpowers.md` — Superpowers (composable skills, subagent-driven dev, code review) ✅

## Phase 2: Memory & Knowledge (2 projects)
> Priority: P0 — Trực tiếp cần cho orchestrator learning

- [x] **T-200**: `DISCOVERY-agentmemory.md` — AgentMemory (persistent memory, hybrid search, MCP tools) ✅
- [x] **T-201**: `DISCOVERY-zep.md` — Zep (knowledge graphs, Go backend, conversation memory) ✅

## Phase 3: Token Compression (6 projects)
> Priority: P1 — Operational cost optimization

- [x] **T-300**: `DISCOVERY-headroom.md` — Headroom (Rust proxy, CCR reversible compression) ✅
- [x] **T-301**: `DISCOVERY-claw-compactor.md` — Claw Compactor (14-stage fusion pipeline, Python) ✅
- [x] **T-302**: `DISCOVERY-rtk.md` — RTK (Rust CLI, 100+ commands, <10ms overhead) ✅
- [x] **T-303**: `DISCOVERY-caveman.md` — Caveman (output token reduction skill, 65% fewer tokens) ✅
- [x] **T-304**: `DISCOVERY-toon.md` — TOON (Token-Oriented Object Notation format) ✅
- [x] **T-305**: `DISCOVERY-llmlingua.md` — LLMLingua (Microsoft prompt compression research) ✅

## Phase 4: Infrastructure (6 projects)
> Priority: P1 — Hạ tầng hỗ trợ

- [x] **T-400**: `DISCOVERY-9router.md` — 9Router (LLM router, cost optimization, SSE proxy) ✅
- [x] **T-401**: `DISCOVERY-openclaw.md` — OpenClaw (multi-channel AI gateway, desktop/mobile apps) ✅
- [x] **T-402**: `DISCOVERY-free-claude-code.md` — Free Claude Code (local proxy, provider abstraction) ✅
- [x] **T-403**: `DISCOVERY-llm-key-manager.md` — LLM Key Manager (hybrid AI gateway, key rotation) ✅
- [x] **T-404**: `DISCOVERY-prompt-base.md` — Prompt Base (modular AI framework, skills/workflows/agents) ✅
- [x] **T-405**: `DISCOVERY-antigravity-awesome-skills.md` — Antigravity Awesome Skills (1900+ skills registry) ✅

## Phase 5: Tổng Hợp
> Priority: P0 — Chạy sau khi hoàn thành Phase 1-4

- [x] **T-500**: Tạo `docs/references/README.md` — Master index + Top 10 Applied Takeaways ✅
- [x] **T-501**: Cross-project comparison table (architecture, testing, DX, cost) ✅

---

## Thứ Tự Thực Hiện

```
Phase 0 (cleanup) → Phase 1 (agent-platform) → Phase 2 (memory)
    → Phase 3 (token-compression) → Phase 4 (infrastructure) → Phase 5 (tổng hợp)
```

**Tổng**: 25 tasks (Phase 0: 3 · Phase 1-4: 20 reports · Phase 5: 2) · **Estimated**: Mỗi report ~15-20 phút phân tích
