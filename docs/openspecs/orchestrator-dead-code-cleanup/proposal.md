# Proposal: Orchestrator Dead Code Cleanup

## Why

Audit toàn bộ `internal/orchestrator` (+ subpackages) phát hiện code không còn call site nào — sản phẩm phụ của các lần refactor trước (execution-engine abstraction đổi hướng, prompt-assembler wiring đổi cách inject, tool-dispatch path đổi từ wrapper method sang gọi registry thẳng). Không liên quan đến `ExecutionProviders`/CLI routing (đã tách riêng ở [`execution-provider-routing-adoption-gaps/`](../execution-provider-routing-adoption-gaps/)) — đây thuần là dọn rác, không đổi hành vi runtime.

State-machine loop legacy (`llmrunner/toolloop.go` vs `statemachineloop.go`) **không nằm trong OpenSpec này** — đã tự document + track ở [`execution-semantics-2026/tasks.md`](../execution-semantics-2026/tasks.md) Task 2.2/3.1 ("pending release cycle to delete legacy loop"), không cần duplicate.

## What Changes

### Issue 1: `engine.apiNativeEngine` chết
- `internal/orchestrator/engine/api_native.go` — `NewAPINativeEngine`/`apiNativeEngine`/`Delegate`: 0 call site sản xuất (chỉ `api_native_test.go` tự gọi). Thiết kế thật (theo comment ở `cli_engine_step.go:153`) là api_native **không** đi qua interface `ExecutionEngine` — trả `nil` từ `resolveCLIEngineRunner` để caller tự fallback `llmRunnerAdapter`. File này là scaffolding cho 1 hướng thiết kế đã bị bỏ.
- Xoá `api_native.go` + `api_native_test.go`.

### Issue 2: `promptAssemblerAdapter` chết
- `internal/orchestrator/service_adapters.go:61-67` — 0 nơi construct. `WithPrompts` (`orchestrator.go:244`) nhận thẳng `*prompts.PromptAssembler` (đã tự thoả interface `PromptBuilder`).
- Xoá type + method liên quan.

### Issue 3: 2 method chết trong `analyze_tools.go`
- `AnalyzeStep.readAnalyzeFile` (`analyze_tools.go:111`), `grepAnalyzeFiles` (`:189`) — chỉ được gọi từ test, không phải từ `executeAnalyzeTool`'s dispatch (khác `listAnalyzeFiles`, method song sinh vẫn được gọi thật ở `analyze.go:559`).
- Cả 3 method chia sẻ ~40 dòng logic "resolve workspace roots từ `TaskWorkspace.Repos`" giống hệt nhau — nếu giữ lại vì lý do khác (vd dispatch surface tương lai), tách phần dùng chung thành helper; nếu xoá, xoá luôn test tương ứng.

## Capabilities

### Removed Capabilities
- `engine.NewAPINativeEngine`/`apiNativeEngine`.
- `orchestrator.promptAssemblerAdapter`.
- `AnalyzeStep.readAnalyzeFile`/`grepAnalyzeFiles` (trừ khi Task 1.3 xác nhận cần giữ + extract helper thay vì xoá).

## Impact

| Area | Files Affected |
|------|----------------|
| Backend orchestrator | `server/internal/orchestrator/engine/api_native.go` (xoá), `server/internal/orchestrator/engine/api_native_test.go` (xoá), `server/internal/orchestrator/service_adapters.go`, `server/internal/orchestrator/steps/analyze_tools.go`, `server/internal/orchestrator/steps/analyze_test.go` |
