# Long-Term Roadmap: Auto Code OS Enhancements (learned from reference projects)

> **Source**: `docs/references/README.md` (20 reference projects, Top-10 takeaways + verified gaps 2026-07-20) + user direction 2026-07-21.
> **Cách đọc**: các hạng mục xếp theo **thứ tự triển khai đề xuất** (P0 → P4). Mỗi wave chỉ bắt đầu khi wave trước đủ ổn định; trong cùng 1 wave các item độc lập, có thể làm song song.
>
> **Decisions locked** (2026-07-21, với user, cho Wave 1):
> - CLI target: **generic configurable command** (không hard-code CLI cụ thể; preset để sau)
> - Flow mới (analyze → openspec → implement → MR) chỉ áp dụng cho **CLI mode**; API-native giữ nguyên DAG hiện tại
> - Toggle: **per-project default + per-task override**
> - CLI subprocess chạy **trong sandbox container** hiện có

---

## 🗺️ Overview — Implementation Order

| Order | Wave | Item | Nguồn học | Impact | Effort | Vì sao ở vị trí này |
|-------|------|------|-----------|--------|--------|---------------------|
| **P0.1** | 0 — Quick wins | Anthropic prompt caching (`cache_control` trong `pkg/llm/anthropic.go`) | headroom, claw-compactor | HIGH | 0.5d | ✅ Done — system+tools cached, cache-token usage logged |
| **P0.2** | 0 | Mở rộng `secretPatterns` memory redaction (AWS/JWT/Google/npm/gh tokens) | agentmemory | MED | 0.5d | ✅ Done — `memory.go:secretPatterns` |
| **P0.3** | 0 | Wire `ApplyDecay()` vào ticker (pattern `cache_workers.go` có sẵn) | agentmemory, zep | MED | 2d | ✅ Done — `MemoryService.StartDecayWorker`, 6h interval, wired trong `main.go` |
| **P0.4** | 0 | Circuit breaker cho `MemoryEmbedder.Embed()` → fallback BM25-only | zep | MED | 1d | ✅ Done — `embedder_breaker.go`, mở sau 5 lỗi liên tiếp, cooldown 2m |
| **P1.1** | 1 — CLI Engine ⭐ | **Pluggable execution engine** — spawner interface + generic subprocess-CLI chạy trong sandbox, UI toggle per-project/per-task → [`pluggable-execution-engine/`](./pluggable-execution-engine/proposal.md) | Multica, ai-sdlc (`SubagentSpawner`) | HIGH | 5-7d | ✅ Done — Engine interface, CLI spawner, preflight, loop-kill, web UI settings & task badges |
| **P1.2** | 1 | **CLI spec-first flow** — analyze → openspec → implement → merge request (chỉ CLI mode) → [`cli-spec-first-flow/`](./cli-spec-first-flow/proposal.md) | OpenSpec, ai-sdlc | HIGH | 4-6d | Phụ thuộc P1.1; là flow chính khi dùng CLI engine |
| **P2.1** | 2 — Chất lượng pipeline | Fuzzy fallback cho `search_replace.go` (whitespace-normalize → relative-indent) | Aider | HIGH | 2-3d | Giảm retry/fail thật sự đang xảy ra ở patch apply |
| **P2.2** | 2 | Definition-of-Ready gate (`dor_check` trước coding) | ai-sdlc | HIGH | 1-2d | Chặn lãng phí token cho task chưa đủ spec |
| **P2.3** | 2 | Review verdict tách đôi: spec-compliance vs code-quality, route khác nhau khi fail | Superpowers | MED | 2-3d | Nâng chất lượng review loop hiện có |
| **P2.4** | 2 | Tool-output filtering pipeline (dedup log, keep-error-lines) trước hard-cut 8000 chars | rtk, claw-compactor | HIGH | 4-5d | Gap token-compression lớn nhất đã verify |
| **P3.1** | 3 — Thông minh hơn | Cross-harness review (provider khác review code của provider đã code; với CLI mode: API-native review CLI output) | ai-sdlc | HIGH | 2-3d | Cần P1 xong để có 2 engine mà cross |
| **P3.2** | 3 | RepoMap mention-boost (×10 identifier nhắc trong task description) | Aider | MED | 2-3d | Cải thiện context selection, độc lập |
| **P3.3** | 3 | Smart LLM router theo complexity (task dễ → model rẻ) + `token_usage` tracking | 9Router | MED | 3-4d | Sau khi có caching (P0.1) số liệu mới có nghĩa |
| **P3.4** | 3 | MMR/diversity dedup sau RRF merge trong memory search | agentmemory | LOW-MED | 1d | Tinh chỉnh, không chặn gì |
| **P4.1** | 4 — Nền tảng dài hạn | Reusable skills system (trích pattern từ task `merged` → skill record, load lại ở context_loading) | Multica, Hermes, Superpowers | HIGH | 5-8d | Giá trị lũy kế lớn nhưng cần pipeline ổn định trước |
| **P4.2** | 4 | Declarative governance schemas (DAG/quality-gate cấu hình bằng JSON schema per-project) | ai-sdlc | MED | 5-7d | Tổng quát hóa những gì P1/P2 đã hard-code |
| **P4.3** | 4 | Attestation & audit trail (DSSE metadata trên PR: agent, model, prompt hash, review chain) | ai-sdlc | MED | 3-4d | Enterprise; càng có ý nghĩa khi đã có 2 engine + cross-review |
| **P4.4** | 4 | Per-N-turn learning nudge (DetectPatterns giữa task, không chỉ end-of-task) | Hermes | MED | 2-3d | Cần P4.1 skills system để tận dụng |

**Đã có sẵn — không làm lại** (verified 2026-07-20): 4-tier memory + RRF hybrid search, worktree-per-task isolation, token counting bằng tiktoken trong repomap, exit-code tách khỏi output text.

## Nguyên tắc xếp thứ tự

1. **Wave 0** — việc nhỏ, lợi chắc chắn, không đợi gì: làm ngay xen kẽ trong khi Wave 1 được spec.
2. **Wave 1 (CLI engine)** — ưu tiên chiến lược do user chọn; là openspec sets đầu tiên được author (P1.1, P1.2 bên dưới).
3. **Wave 2** — sửa các điểm đau đã đo được của pipeline API-native hiện tại; song song được với Wave 1 vì không đụng nhau.
4. **Wave 3** — các tính năng "thông minh" chỉ có nghĩa khi nền Wave 0-2 xong (cross-review cần 2 engine, router cần số liệu caching).
5. **Wave 4** — đầu tư dài hạn, giá trị lũy kế, chưa spec chi tiết — sẽ author openspec khi đến lượt.

## OpenSpec sets (đã author đủ — 2026-07-21)

| Set | Phase | Trạng thái |
|-----|-------|-----------|
| [`llm-prompt-caching/`](./llm-prompt-caching/proposal.md) | P0.1 | 📝 Authored |
| [`memory-hardening/`](./memory-hardening/proposal.md) | P0.2–P0.4 + P3.4 (gộp — cùng memory service) | 📝 Authored |
| [`pluggable-execution-engine/`](./pluggable-execution-engine/proposal.md) | P1.1 | ✅ Done |
| [`cli-spec-first-flow/`](./cli-spec-first-flow/proposal.md) | P1.2 | ✅ Done — all tasks.md sections (1-6) complete |
| [`search-replace-fuzzy-fallback/`](./search-replace-fuzzy-fallback/proposal.md) | P2.1 | ✅ Done — all tasks.md items complete (1.1/1.9 skipped, no corpus available) |
| [`definition-of-ready-gate/`](./definition-of-ready-gate/proposal.md) | P2.2 | ✅ Done — round-limit/hotfix bypass added; REQ-004b + UI badge skipped (see tasks.md) |
| [`review-verdict-split/`](./review-verdict-split/proposal.md) | P2.3 | 📝 Authored |
| [`tool-output-filtering/`](./tool-output-filtering/proposal.md) | P2.4 | 📝 Authored |
| [`cross-harness-review/`](./cross-harness-review/proposal.md) | P3.1 | 📝 Authored |
| [`repomap-mention-boost/`](./repomap-mention-boost/proposal.md) | P3.2 | 📝 Authored |
| [`smart-llm-router/`](./smart-llm-router/proposal.md) | P3.3 | 📝 Authored |
| [`reusable-skills-system/`](./reusable-skills-system/proposal.md) | P4.1 + P4.4 (gộp — cùng learning pipeline) | 📝 Authored |
| [`declarative-governance-schemas/`](./declarative-governance-schemas/proposal.md) | P4.2 | 📝 Authored |
| [`attestation-audit-trail/`](./attestation-audit-trail/proposal.md) | P4.3 | 📝 Authored |
| [`feature-docs-sync/`](./feature-docs-sync/proposal.md) | Cross-cutting (làm sớm, song song Wave 0) — chống outdate cho `docs/features/` | 📝 Authored |

Ghi chú phối hợp giữa các set:
- **Pause/resume helper** dùng chung bởi `definition-of-ready-gate`, `cli-spec-first-flow` (spec approval), `review-verdict-split` (escalation) — set nào implement trước thì tạo helper, các set sau tái dùng.
- **Prerequisites**: `smart-llm-router` ← `llm-prompt-caching`; `cross-harness-review` ← `pluggable-execution-engine` + `review-verdict-split`; `attestation-audit-trail` ← `cross-harness-review`; `cli-spec-first-flow` ← `pluggable-execution-engine`; `declarative-governance-schemas` ← Wave 1-3.
- **Docs sync**: mỗi set khi ship phải thực hiện mapping trong `feature-docs-sync/design.md`.

## Key risks (Wave 1)

1. **CLI có sẵn trong sandbox image không** — preflight check với message rõ ràng thay vì lỗi step khó hiểu.
2. **Auth/credentials cho CLI** — env vars per-project engine config, lưu như repo credentials, không log.
3. **Black-box tool-loop** — mất per-tool-call observability (trade-off chấp nhận theo reference analysis); bù bằng full stdout/stderr thành step logs + git diff để review thay đổi.
