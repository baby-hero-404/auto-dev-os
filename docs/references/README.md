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

### 1. **~~Agent Persistent Memory System~~ → Already built; close the verified gaps instead** (from agentmemory + zep)
> ⚠️ **Outdated as written** — verified 2026-07-20 against `server/internal/service/memory.go` + `memory_search.go`. Auto Code OS **already has** a 4-tier memory system (`working/episodic/semantic/procedural`) with triple-stream RRF search (BM25 + vector + graph, `rrfK=60`, same constant as agentmemory) and Ebbinghaus decay. Do not rebuild this — see [Confirmed Gaps & Quick Wins](#-confirmed-gaps--quick-wins-verified-against-source-2026-07-20) below for the concrete, verified deltas (unwired decay sweep, incomplete secret regex, missing embedder circuit breaker, no MMR dedup on search results).
- **Impact**: HIGH (fixing the gaps, not the whole system) · **Effort**: LOW-MEDIUM · **Risk**: LOW · **Est**: 0.5-2 days per gap

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

## 🔬 Confirmed Gaps & Quick Wins (verified against source, 2026-07-20)

> Unlike the Top 10 above (written at report-creation time, some now stale), every item here was re-verified directly against current `server/` source — grep'd, read, and cross-checked — as part of a deepening pass. Ordered by user-requested priority: **agent-platform → memory → token-compression**. Each entry states what's real vs. assumed.

### Memory
1. **Anthropic prompt caching is completely absent** — `grep -rn "cache_control" server/pkg/llm` returns nothing. This is a prerequisite for 2 separate takeaways (headroom's cache-drift detection, claw-compactor's QuantumLock prefix stabilization) that both assumed caching already existed. Fix first: add `cache_control: {type: ephemeral}` to the system+tools block in `server/pkg/llm/anthropic.go:51` (`ChatWithOptions`). Everything else about prompt-prefix stability is moot until this exists. Est: 0.5 day.
2. **`MemoryService.ApplyDecay()` has zero callers anywhere in the codebase** (`server/internal/service/memory.go:136`) — confirmed via repo-wide grep. The decay logic itself (`UpdateDecay`, `repository/memory.go:198`) only does `decay_score *= 0.95` on stale rows; it has no contradiction-detection or TTL sweep. Needs both a ticker (pattern already exists at `orchestrator/cache_workers.go:19`) and a richer decay function. Est: 2 days.
3. **`secretPatterns` in `memory.go:212-217` is only 5 regexes** — missing AWS `AKIA`, Google `AIza`, JWT, `npm_`, and GitHub's newer token prefixes (`ghs_/ghu_/github_pat_`, only `ghp_` is covered). Memory captures raw tool output, so this is a real leak surface. Est: 0.5 day.
4. **No circuit breaker around `MemoryEmbedder.Embed()`** (`server/pkg/llm/embedding.go`, 84-line plain HTTP call) — a rate-limited/down embedding provider fails `RecordObservation`/`Search` outright instead of falling back to BM25-only. Est: 1 day.
5. **No MMR/diversity step after RRF merge** (`memory_search.go:78`, `rrfMerge`) — top-N results can be near-duplicate content, wasting context tokens. Est: 1 day.

### Token Compression
1. **Zero output-filtering layer for tool results** — `grep -rln "Truncate\|MaxLen\|maxOutput" server/internal/tool` returns nothing; the only bound anywhere is a blunt 8000-char hard cut in `orchestrator/llmrunner/toolloop.go:44-58` (`maxToolResultChars`). No content-aware filtering (log dedup, keep-error-lines) exists before that cut. This is the single biggest token-compression gap. Est: 4-5 days for a first filter pipeline (see rtk/claw-compactor reports).
2. **Exit-code handling is already clean** — `res.ExitCode` (`tool/tools/run_build.go`, `run_lint.go`, `git_diff.go`, `git_status.go`) is a field fully separate from output text, so adding output compression later carries no risk of corrupting status semantics. No action needed here.
3. **Token counting is already solid where it exists** — `repomap/pruning.go` uses a real binary search over `tiktoken-go` (cl100k_base) counts, matching (and in one respect exceeding — exact tokenizer vs. Aider's 1%-sample estimate) the reference implementations. Don't touch this; extend the same rigor to tool-output compression instead of reinventing token estimation there.

### Agent Platform
1. **`search_replace.go` is exact-match-only with zero fallback** — a single whitespace/indent mismatch in an LLM-generated patch hard-fails immediately (`ApplySearchReplace`, `orchestrator/patch/search_replace.go:97-140`, `strings.Count` + immediate `return fmt.Errorf`). Aider's multi-tier fuzzy fallback (whitespace-normalize → relative-indent → diff-match-patch) is a concrete, scoped fix. Est: 2-3 days.
2. **RepoMap PageRank already matches Aider's active-file boost (50x) but has no mention-boost** — `context/repomap/ranking.go` independently arrived at the same 50x multiplier Aider uses for active files; the missing piece is boosting edges for identifiers mentioned in the task description (Aider's `mentioned_idents` ×10). Est: 2-3 days.
3. **`learning.DetectPatterns` only fires at end-of-task, confirmed** — wired in `orchestrator/worker.go:566`, gated on `finalStatus == WorkflowJobStatusDone`. Hermes-agent's per-N-turn nudge pattern is a legitimate enhancement, not a "does this exist" question — it's confirmed absent mid-task. Est: 2-3 days.
4. **Review step has a single verdict field, not split spec/quality** — `server/internal/prompts/steps/review.md` has no `spec_compliance`/`code_quality`/`verdict` fields (confirmed via grep). Superpowers' 2-verdict split (spec compliance vs. code quality, routed differently on failure) is a real, unimplemented gap. Est: 2-3 days.

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

### 🔌 Mô Hình Tích Hợp LLM (4 Kiến Trúc)

Sau khi đào sâu cách từng project "nói chuyện" với LLM, có 4 mô hình tích hợp phân biệt rõ theo ai giữ tool-loop và ai trả tiền API:

| Mô hình | Định nghĩa | Ai giữ tool-loop | Projects | Trade-off chính |
|---------|-----------|-------------------|----------|-----------------|
| **Subprocess-CLI** | Spawn 1 CLI coding agent đã cài sẵn (Claude Code, Codex CLI...) như subprocess, server chỉ điều phối + stream log | CLI tool bên thứ 3 (black-box) | Multica (`exec.CommandContext` trong worktree cô lập); ai-sdlc Tier 2 qua `ShellClaudePSpawner` | Không tự trả token API (user tự subscription), nhưng mất kiểm soát prompt/tool-loop, phụ thuộc CLI đã cài đúng version |
| **SDK-embedded** | Import SDK chính thức của 1 provider (vd Claude Agent SDK) vào process của mình, dùng session/tool-loop do SDK quản lý | SDK (do provider viết, nhưng chạy in-process) | ai-sdlc Tier 1 qua `ClaudeCodeSDKSpawner`; ai-sdlc Tier 1 slash-command dùng `Agent` tool trực tiếp trong Claude Code session | Ít code tự viết hơn subprocess, nhưng khoá cứng vào 1 provider, khó multi-model |
| **API-native** | Tự viết toàn bộ tool-loop (build prompt, gọi Messages/Chat Completions API, parse tool call, apply edit) — không phụ thuộc CLI hay SDK agent nào | Chính mình (full control) | Aider (`base_coder.py` → `litellm`); Hermes Agent (`agent/transports/*.py` adapter pattern cho Anthropic/OpenAI/Bedrock/Gemini); **Auto Code OS hiện tại** (`server/internal/orchestrator/llmrunner/toolloop.go` → `server/pkg/llm/`) |Toàn quyền kiểm soát prompt/cost/retry, nhưng phải tự bảo trì tool-loop + tự trả token API trực tiếp qua key |
| **Hybrid (pluggable spawner)** | Strategy/adapter pattern cho phép switch giữa các mô hình trên bằng dependency injection, không hard-code 1 cách gọi LLM | Tuỳ implementation được inject | ai-sdlc — interface `SubagentSpawner` với 3 implementation (`ShellClaudePSpawner` = subprocess-CLI, `ClaudeCodeSDKSpawner` = SDK-embedded, `MockSpawner` = test) chọn qua injection | Linh hoạt nhất (subscription billing vs API-key billing vs test hermetic), nhưng bề mặt bảo trì lớn hơn (nhiều adapter cùng hợp đồng phải đồng bộ) |

**Auto Code OS đang ở mô hình API-native** (giống Aider/Hermes Agent) — tự viết tool-loop, tự gọi LLM API trực tiếp. Điểm khác biệt so với Multica/ai-sdlc: không có lựa chọn "outsource" trí tuệ cho CLI agent người dùng tự cài. Nếu muốn thêm lựa chọn subprocess-CLI trong tương lai (vd cho phép user dùng subscription Claude Code của họ thay vì API key trả riêng), nên theo mẫu `SubagentSpawner` interface của ai-sdlc thay vì hard-code — giữ API-native làm default, subprocess-CLI làm 1 implementation thay thế sau lưng cùng interface.

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
