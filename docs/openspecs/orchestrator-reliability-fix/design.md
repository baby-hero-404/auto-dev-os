# Design: Orchestrator & Prompt Reliability Fix

> **Evidence Source:** Real task trace `72d0ff65` — 13 LLM calls, 10 consecutive retry failures.

---

## Context

Hệ thống Auto Code OS hiện đang ở Phase 12 (Patch Engine Abstraction). Orchestrator đã có cơ chế đầy đủ: Analyze → Plan → Code (parallel) → Merge → Review → Fix → Test → PR. Tuy nhiên, khi chạy task thực tế với complexity `hard` (6 execution units), hệ thống **thất bại hoàn toàn ở giai đoạn coding song song** do 3 loại lỗi chồng chéo:

1. **Infrastructure:** Agent lock leak + Credential exhaustion (không có fallback path)
2. **Prompt Quality:** Coding steps nhận prompt thiếu context (repo structure rỗng, full manifest thừa)
3. **Workflow Resilience:** Retry storm không có backoff + re-execute steps đã thành công

### Dữ liệu thực nghiệm (Task `72d0ff65`):

| Metric | Value |
|--------|-------|
| Total LLM calls | 13 |
| Analyze calls | 5 (1 tool call + 3 clarification rounds + 1 final) |
| Coding calls | 8 |
| Unique successful coding outputs | 3 (call-006, call-008, call-012) |
| Wasted coding calls (retries/failures) | 5 |
| Total retry cycles | ~10 (timeline events) |
| Agent lock failures | 7 lần liên tiếp |
| Credential exhaustion failures | 3 lần liên tiếp |
| Token waste (estimated) | ~40,000 tokens (re-running code_backend_1 x5) |
| Final status | `failed` |

### Execution Flow Diagram:

```
context_load ──► analyze (5 LLM calls, 3 Q&A rounds) ──► plan (1s, no-op)
                                                              │
                                                    ┌────────┼────────┐
                                                    ▼        ▼        ▼
                                              backend_0  backend_1  backend_2
                                              (SUCCESS)  (SUCCESS   (FAIL x10)
                                                          x5 runs)
                                                                      │
                                                              ┌───────┴────────┐
                                                              ▼                ▼
                                                     credential      agent lock
                                                     exhaustion      (stale)
                                                     (3x)            (7x)
```

---

## Goals / Non-Goals

**Goals:**
1. Đảm bảo task Hard với parallel coding steps có thể hoàn thành end-to-end.
2. Loại bỏ agent resource leak để retry workflow thực sự có hiệu quả.
3. Giảm token waste ≥50% bằng cách prune prompt và không re-execute completed steps.
4. Cung cấp đủ workspace context cho mỗi coding step (file listing, path conventions).

**Non-Goals:**
- Refactor toàn bộ Prompt Assembly thành Deterministic Compilation (Phase tương lai).
- Triển khai AST-based editing (đã có trong backlog Phase 12).
- Thay đổi DAG workflow definition format.
- Multi-model routing intelligence (đã có qua AI Gateway).

---

## Decisions

### D-1: Agent Release — Defensive Double-Release Pattern
**Quyết định:** Giữ nguyên `defer` release ở top-level `worker.go` (line 52) VÀ thêm release trong `code_backend.go` defer block khi `assignedAgentID != ""`. Double-release sẽ được bảo vệ bằng idempotent `UpdateStatus` (set idle nếu đã idle = no-op).

**Rationale:** Hiện tại `code_backend.go` line 104-110 đã có defer release cho agent được assign bởi `assignByRole`, nhưng nếu step thất bại ở dòng 93 (trước khi defer được thiết lập), agent bị leak. Cần đảm bảo release xảy ra ở MỌI exit path.

### D-2: Credential Wait-and-Retry — Bounded Wait
**Quyết định:** Khi gateway nhận `exhausted routes`, thay vì fail ngay, gateway sẽ kiểm tra cooldown thấp nhất. Nếu cooldown < 30s, wait và retry. Nếu cooldown ≥ 30s, fail ngay (để workflow retry ở level cao hơn).

**Rationale:** Cooldown hiện tại mặc định là 60s. Credential test failure (400/401) set cooldown 5 phút. Wait 30s là hợp lý cho rate-limit cooldown nhưng không nên block thread cho auth failure cooldown.

### D-3: Prompt Pruning — Tiered Context Strategy
**Quyết định:** Chia prompt thành 3 tiers:
- **Tier 1 (Always):** System prompt, Global rules, Task title, Assigned subtask, Q&A answers
- **Tier 2 (Role-specific):** Design section, Repository structure (live), Affected files from prior steps
- **Tier 3 (On-demand):** Full Execution Manifest, Risk details, Acceptance criteria — chỉ inject cho Analyze/Review steps

**Rationale:** Coding step prompt hiện tại ~4200 tokens, trong đó ~2000 tokens là Tier 3 content không cần thiết. Cắt giảm ~48% token = giảm cost + ít nhiễu cho LLM.

### D-4: Workspace File Listing — Live Scan at Step Start
**Quyết định:** Trước khi gọi LLM cho coding step, thực hiện `ls -R` hoặc `git ls-files` trên worktree thực tế để sinh danh sách file hiện tại. Inject vào `=== Repository Structure ===`.

**Rationale:** Hiện tại section này là snapshot tĩnh từ `context_load` step (chỉ có `.git:`). Sau `code_backend_0` tạo ~10 files, `code_backend_1` vẫn nhận `.git:` rỗng → LLM phải đoán.

### D-5: Checkpoint-Aware Retry — Skip Completed Steps
**Quyết định:** Khi workflow retry, engine PHẢI kiểm tra checkpoint của mỗi step. Nếu step đã có checkpoint `success`, skip hoàn toàn (không re-execute, không re-assign agent).

**Rationale:** Hiện tại `engine.CompletedSteps` đã được populate từ checkpoints, nhưng parallel step groups có vẻ không respect checkpoint khi graph bị rebuilt. Evidence: `code_backend_1` chạy lại 5 lần dù checkpoint ghi success ở lần đầu.

---

## Risks / Trade-offs

| Risk | Probability | Severity | Mitigation |
|------|-------------|----------|------------|
| **Double-release agent** gây race condition khi 2 goroutine cùng release | Low | Low | `UpdateStatus` với WHERE clause `status IN ('assigned','running')` là idempotent. Rows affected = 0 nếu đã idle. |
| **Credential wait** block worker goroutine | Medium | Medium | Bounded wait (max 30s) + context cancellation support. Worker có semaphore concurrency limit nên block 1 slot = giảm throughput nhưng không deadlock. |
| **Live file scan** chậm trên workspace lớn | Low | Low | Giới hạn depth=3, exclude `.git/`, truncate ở 200 entries. Workspace mới chỉ có vài chục file. |
| **Prompt pruning** loại bỏ info mà LLM cần | Medium | High | Giữ Design section (goals, decisions, constraints) trong Tier 2. Chỉ loại Execution Manifest raw JSON (acceptance_criteria, risk_domains) — info này đã được diễn giải trong OpenSpec text sections. |
| **Skip completed steps** trong parallel group gây inconsistency | Medium | High | Chỉ skip nếu checkpoint SpecHash khớp với current SpecHash. Nếu spec thay đổi, invalidate checkpoint và chạy lại. |

---

## Open Questions

1. **Plan Step:** Có nên loại bỏ hoàn toàn Plan step khi Analyze đã sinh execution_units? Hay refactor Plan step để validate/refine DAG dựa trên workspace state?
2. **Parallel Step Ordering:** Khi `code_backend_0` (setup) phải chạy trước `code_backend_1` và `code_backend_2`, liệu có nên enforce dependency ở DAG level thay vì dựa vào `parallelizable: false` flag trong execution_units?
3. **Worktree Isolation:** Hiện tại parallel steps dùng separate worktrees. Khi `code_backend_0` commit vào main branch, các worktree khác có tự động nhận files mới không? Hay cần merge trước?

---

## Priority Roadmap

| Phase | Issues | Effort | Impact |
|-------|--------|--------|--------|
| **P0: Agent & Retry Fix** | ISSUE-1 (agent leak), ISSUE-6 (retry storm) | Small (1-2 files) | Critical — unblocks all Hard tasks |
| **P1: Prompt Quality** | ISSUE-3 (context sharing), ISSUE-4 (pruning), ISSUE-7 (path clarity) | Medium (3-4 files) | High — reduces token waste ~50%, improves LLM output quality |
| **P2: Gateway Resilience** | ISSUE-2 (credential wait) | Small (1 file) | Medium — eliminates credential-related failures |
| **P3: Plan Refinement** | ISSUE-5 (plan effectiveness) | Medium (2-3 files) | Low — optimization, not blocking |
