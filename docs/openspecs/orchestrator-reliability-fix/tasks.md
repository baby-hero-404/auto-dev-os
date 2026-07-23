# Task List: Orchestrator & Prompt Reliability Fix

> **Derived from:** [proposal.md](proposal.md) | [specs.md](specs.md) | [design.md](design.md)
> **Evidence:** Task trace `72d0ff65` — 13 LLM calls, 10 retry failures, final status `failed`.

---

## P0: Agent & Retry Fix (Critical — Unblocks all Hard tasks) ✅

### Task 1.1: Agent Step-Level Release on Failure ✅
**Issue:** ISSUE-1 | **File:** `server/internal/orchestrator/steps/code_backend.go`
**Status:** ✅ Completed.

- [x] Thêm `agentRepo.ResetAllStatuses()` khi server khởi động (`cmd/api/main.go`)
- [x] Đảm bảo `defer release` trong `code_backend.go` bao phủ trường hợp `assignByRole` thất bại trước khi defer được thiết lập (line 93-95)
- [x] Áp dụng cùng pattern cho `code_frontend.go` và `review.go` (cả 3 file đều dùng `assignByRole`)
- [x] Thêm unit test: khi `AssignBackendAgent` trả lỗi, agent đã claim trước đó phải được release

**Acceptance:** Sau khi step fail, query `SELECT status FROM agents WHERE role='backend'` phải trả về `idle` trong vòng 1 giây.

---

### Task 1.2: Agent Heartbeat Timeout (Background Watchdog) ✅
**Issue:** ISSUE-1 | **File:** `server/internal/orchestrator/worker.go`, `server/cmd/api/main.go`
**Status:** ✅ Completed.

- [x] Tạo goroutine `AgentWatchdog` chạy mỗi 5 phút
- [x] Query agents có `status IN ('assigned','running')` và `updated_at < NOW() - interval '30 minutes'`
- [x] Reset các agent đó về `idle` và ghi audit log
- [x] Đăng ký watchdog trong `main.go` (tương tự `StartCooldownWorker`)
- [x] Thêm unit test với mock repo

**Acceptance:** Agent bị kẹt `running` > 30 phút tự động được giải phóng.

---

### Task 1.3: Retry Exponential Backoff ✅
**Issue:** ISSUE-6 | **File:** `server/internal/orchestrator/worker.go`
**Status:** ✅ Completed.

- [x] Thay đổi retry delay từ `time.Sleep(2 * time.Second)` (line 322) sang exponential backoff: `min(2^attempt * 2s, 60s)`
- [x] Thêm log ghi rõ delay giữa các lần retry: `"Retrying in %ds..."`
- [x] Thêm unit test: verify delay sequence là 2s, 4s, 8s, 16s, 32s, 60s

**Acceptance:** Timeline log cho thấy delay tăng dần giữa các retry, không còn retry storm (cách nhau 2s liên tục).

---

### Task 1.4: Prevent Re-execution of Completed Parallel Steps ✅
**Issue:** ISSUE-6 | **File:** `server/internal/workflow/engine.go`
**Status:** ✅ Completed.

- [x] Khi workflow retry và rebuild DAG, kiểm tra `engine.CompletedSteps` cho mỗi step trong parallel group
- [x] Nếu step đã có checkpoint `success` VÀ `spec_hash` khớp, skip hoàn toàn (không assign agent, không gọi LLM)
- [x] Ghi log: `"Skipping step %s — already completed (checkpoint found)"`
- [x] Thêm integration test: workflow với 3 parallel steps, step_2 fail → retry → verify step_0 và step_1 không chạy lại

**Acceptance:** Khi retry task `72d0ff65`, `code_backend_1` không chạy lại nếu đã có checkpoint success.

---

## P1: Prompt Quality (High — Reduces ~50% token waste)

### Task 2.1: Live Repository Structure Scan ✅
**Issue:** ISSUE-3, ISSUE-7 | **Files:** `server/internal/orchestrator/steps/code_backend.go`, `server/internal/orchestrator/steps/code_frontend.go`
**Status:** ✅ Completed.

- [x] Trước khi gọi `RunLLMStep`, thực hiện filesystem scan trên worktree path (depth=3, exclude `.git/`)
- [x] Format kết quả thành tree text và inject vào prompt section `=== Repository Structure ===`
- [x] Giới hạn tối đa 200 entries, truncate với message `"... and N more files"`
- [x] Thêm unit test với mock filesystem

**Acceptance:** Prompt của `code_backend_1` chứa danh sách file thực tế (bao gồm files tạo bởi `code_backend_0`).

---

### Task 2.2: Workspace Path Instruction Injection ✅
**Issue:** ISSUE-7 | **File:** `server/internal/orchestrator/steps/code_backend.go` (line 147-182)
**Status:** ✅ Completed.

- [x] Khi workspace chỉ có 1 repo, thêm instruction rõ ràng:
  ```
  IMPORTANT: Your workspace root IS the repository root.
  All file paths MUST be relative (e.g., internal/model/commit.go).
  Do NOT prefix with the repository name.
  ```
- [x] Áp dụng cùng pattern cho `code_frontend.go` và `fix.go`
- [x] Thêm Patch Validator warning khi LLM output chứa file path có prefix trùng repo name

**Acceptance:** LLM không còn tạo file ở 3 prefix khác nhau (`tool_zentao/`, `/`, `zentao-auto-tool/`).

---

### Task 2.3: Prompt Pruning — Remove Full Manifest from Coding Steps ✅
**Issue:** ISSUE-4 | **File:** `server/internal/prompts/assembler.go` hoặc `server/internal/orchestrator/llm_step.go`
**Status:** ✅ Completed.

- [x] Xác định vị trí inject Execution Manifest JSON vào prompt (search `Execution Manifest`)
- [x] Tạo flag/config để phân biệt step types: `analyze`/`review` (full manifest) vs `code_*`/`fix` (pruned)
- [x] Cho coding steps, chỉ inject:
  - OpenSpec text sections (Why, What Changes, Capabilities, Design)
  - Assigned subtask
  - Q&A answers
- [x] Loại bỏ khỏi coding prompts:
  - Raw `acceptance_criteria` array
  - `risk_domains` array
  - `execution_boundaries` array
  - `execution_phases` array
  - `risks` / `risks_details` arrays
- [x] Thêm log metric: `prompt_tokens_before` vs `prompt_tokens_after` để verify giảm ~48%
- [x] Thêm unit test: verify coding step prompt không chứa `acceptance_criteria`

**Acceptance:** Coding step prompt giảm từ ~4200 tokens xuống ~2200 tokens.

---

### Task 2.4: Inter-Step File Change Propagation ✅
**Issue:** ISSUE-3 | **Files:** `server/internal/orchestrator/steps/code_backend.go`, `server/internal/workflow/engine.go`
**Status:** ✅ Completed.

- [x] Khi coding step hoàn thành, extract `files_changed` từ LLM output và lưu vào `stepCtx.Outputs`
- [x] Khi coding step tiếp theo nhận prompt, đọc `files_changed` từ các step trước (via `stepCtx.Inputs`)
- [x] Inject vào prompt section: `### Files Created/Modified by Prior Steps ###`
- [x] Thêm integration test: verify `code_backend_1` prompt chứa files từ `code_backend_0` output

**Acceptance:** `code_backend_1` biết `code_backend_0` đã tạo `go.mod`, `main.go`, `config/config.go`.

---

## P2: Gateway Resilience (Medium — Eliminates credential failures)

### Task 3.1: Credential Wait-and-Retry ✅
**Issue:** ISSUE-2 | **File:** `server/internal/gateway/ai_gateway.go`
**Status:** ✅ Completed.

- [x] Khi gateway nhận `exhausted routes` error, kiểm tra cooldown thấp nhất của tất cả credential cho provider
- [x] Nếu cooldown < 30s: `time.Sleep(cooldown)` rồi retry (tối đa 1 lần)
- [x] Nếu cooldown ≥ 30s: trả về error ngay (để workflow retry ở level cao hơn)
- [x] Support context cancellation trong wait period
- [x] Ghi log: `"All credentials in cooldown. Waiting %ds for credential %s..."`
- [x] Thêm unit test với mock credential pool

**Acceptance:** Khi credential cooldown 10s, gateway tự chờ và retry thay vì fail ngay.

---

### Task 3.2: Credential Recovery Logging ✅
**Issue:** ISSUE-2 | **File:** `server/internal/gateway/cooldown.go` hoặc `server/internal/service/credential_pool.go`
**Status:** ✅ Completed.

- [x] Khi `StartCooldownWorker` reset cooldown cho credential, ghi log `info`: `"Credential %s recovered from cooldown, now available"`
- [x] Thêm metric counter cho credential recovery events

**Acceptance:** Log cho thấy rõ ràng khi credential trở lại available.

---

## P3: Plan Step Refinement (Low — Optimization) ✅

### Task 4.1: Skip Plan Step When Analyze Provides Execution Units ✅
**Issue:** ISSUE-5 | **Files:** `server/internal/orchestrator/steps/plan.go`, `server/internal/workflow/definitions.go`
**Status:** ✅ Completed.

- [x] Trong `PlanStep.Execute()`, kiểm tra nếu Analyze output đã chứa `execution_units` với đầy đủ `dependencies` và `tasks`
- [x] Nếu đã đủ: skip LLM call, trả output ngay từ Analyze data, ghi log `"Plan step skipped — execution units already provided by analyze step"`
- [x] Nếu chưa đủ: chạy LLM call như bình thường
- [x] Thêm unit test cho cả 2 paths

**Acceptance:** Plan step không tốn LLM call khi Analyze đã cung cấp execution_units đầy đủ.

---

### Task 4.2: Enforce Execution Unit Dependencies in DAG ✅
**Issue:** ISSUE-5 | **File:** `server/internal/workflow/dynamic_dag.go`
**Status:** ✅ Completed.

- [x] Khi build DAG từ `execution_units`, respect `parallelizable: false` constraint
- [x] Unit đầu tiên (`setup-project`) luôn chạy trước các unit khác dù chúng khai báo parallel
- [x] Verify DAG ordering matches `dependencies` array trong execution_units
- [x] Thêm test: unit với `dependencies: ["setup-project"]` không chạy cho đến khi `setup-project` success

**Acceptance:** `code_backend_1` và `code_backend_2` không bắt đầu cho đến khi `code_backend_0` (setup) hoàn thành.

---

## Summary

| Priority | Tasks | Estimated Effort | Impact |
|----------|-------|-----------------|--------|
| **P0** | 4 tasks (1.1 → 1.4) | 1-2 ngày | 🔴 Unblocks Hard tasks |
| **P1** | 4 tasks (2.1 → 2.4) | 2-3 ngày | 🟠 -50% token waste, +quality |
| **P2** | 2 tasks (3.1 → 3.2) | 0.5 ngày | 🟡 No credential failures |
| **P3** | 2 tasks (4.1 → 4.2) | 1 ngày | 🟢 Optimization |
| **Total** | **12 tasks** | **~5-6 ngày** | — |

### Dependency Graph

```
Task 1.1 ─┐
Task 1.2 ─┼──► P0 Done ──► Task 2.1 ──► Task 2.4
Task 1.3 ─┤              ├──► Task 2.2
Task 1.4 ─┘              └──► Task 2.3
                          
Task 3.1 ──► Task 3.2    (Independent — can run parallel with P1)

Task 4.1 ──► Task 4.2    (Independent — can run parallel with P1/P2)
```

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
