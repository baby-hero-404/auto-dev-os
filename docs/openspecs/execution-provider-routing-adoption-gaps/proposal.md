# Proposal: Execution Provider Routing — Adoption Gaps

> **Follow-up to**: [`cli-execution-provider-routing/`](../cli-execution-provider-routing/) — đó là OpenSpec gốc thêm `Project.ExecutionProviders` + `ResolveExecutionProvider`. OpenSpec này **không** thay đổi kiến trúc Router, chỉ hoàn thiện 3 điểm gọi còn sót lại vẫn đọc thẳng `ExecutionEngine`/`CLIEngineConfig` cũ thay vì đi qua Router — phát hiện qua audit toàn bộ backend + qua chính việc dùng thử tính năng (session hôm nay đã tự sửa 2 gap tương tự ở `execution_router.go`/`create-task-panel.tsx`, đây là 3 gap còn lại cùng nguyên nhân gốc).

## Why

`design.md` của OpenSpec gốc tự ghi rõ phạm vi chèn kiến trúc: *"Insertion point is intentionally narrow: only `resolveCLIEngineRunner` changes its data source. `step_registry.go:26` and `worker.go:309` keep calling it exactly as before — they don't know the Router exists."* Điều đó đúng cho `code_backend`/`code_frontend`/`fix` (dùng `cliEngineRunner`), nhưng để lại 3 điểm khác **không được nhắc tới trong design.md** vẫn đọc thẳng field cũ:

1. `cliStepRunner.resolveConfig` (`cli_spec_step.go`) — dùng bởi toàn bộ flow `cli_analyze → cli_spec → cli_implement` (xem [`cli-spec-first-flow/`](../cli-spec-first-flow/)) — unmarshal thẳng `project.CLIEngineConfig`, không gọi `ResolveExecutionProvider`.
2. `worker.go:296-309` — chọn `CLISpecFirstWorkflow` (workflow shape) chỉ dựa vào `cliengine.ResolveEngine(task.ExecutionEngine, project.ExecutionEngine)`, không biết `project.ExecutionProviders` tồn tại.
3. `TaskService.validateTaskEngineOverride` (`internal/service/task.go:94-110`) — validate override `execution_engine="cli"` ở tầng task chỉ bằng cách check `project.CLIEngineConfig`, không check `project.ExecutionProviders`.

Hệ quả thực tế (đã verify từng cái):
- Project chỉ cấu hình CLI qua **Execution Providers list mới** (không đụng field `ExecutionEngine`/`CLIEngineConfig` cũ — chính là setup mà 2 fix trước đó trong session này hướng người dùng tới):
  - Không bao giờ được chọn `CLISpecFirstWorkflow` — `worker.go` luôn thấy `ResolveEngine(...) == "api_native"` nên rơi vào DAG mặc định. Trong DAG đó, `code_backend`/`code_frontend`/`fix` vẫn chạy CLI đúng (qua Router), nhưng flow spec-first (analyze→spec→implement với gate approve) không bao giờ kích hoạt được — mâu thuẫn với kỳ vọng "tôi đã bật CLI cho project".
  - Nếu project đó **cũng** dùng `cli_spec_first` (project cũ có `ExecutionEngine="cli"` rồi sau đó thêm `ExecutionProviders`), `cliStepRunner` vẫn đọc `CLIEngineConfig` cũ — bỏ qua hoàn toàn priority/fallback/credential-pin của `ExecutionProviders`, và **không bao giờ set `CredentialID`** nên REQ-006 (write-side cooldown khi quota exceeded) không hoạt động cho toàn bộ flow spec-first — một CLI credential hết quota giữa `cli_analyze`/`cli_spec`/`cli_implement` sẽ không bao giờ được cooldown, Router lần sau vẫn chọn lại đúng credential đó.
  - Gọi API `PATCH /tasks/:id` với `execution_engine="cli"` (override ở task) bị từ chối 400 `"project has no cli_engine_config configured"` dù project đã bật đủ CLI qua Execution Providers — trải nghiệm y hệt bug UI đã sửa hôm nay (`create-task-panel.tsx`), nhưng ở phía server, không phải UI.

Cả 3 đều là cùng một lớp lỗi: **code viết trước khi `ExecutionProviders` tồn tại, chưa được cập nhật để biết field mới**. Không phải thiết kế lại — chỉ là hoàn thiện việc migrate sang Router đã có sẵn.

## What Changes

### Issue 1: `cliStepRunner.resolveConfig` đi qua Router
- `resolveConfig` gọi `o.ResolveExecutionProvider(ctx, task, project)` thay vì unmarshal thẳng `project.CLIEngineConfig`. Nếu `resolved.Type != "cli"`, trả lỗi rõ ràng (route sai — không nên tới đây nếu `worker.go` đã chọn đúng, xem Issue 2) thay vì âm thầm chạy với config rỗng.
- `cliStepRunner` gains `credID string` field (giống `cliEngineRunner`), set từ `resolved.CredentialID`.
- `RunCLIStep` thêm nhánh cooldown giống `cliEngineRunner.RunLLMStep`: `res.QuotaExceeded && r.credID != "" && r.o.cooldownSetter != nil` → `SetCooldown`.

### Issue 2: `worker.go` chọn workflow shape qua Router
- Hàm mới `shouldUseCLISpecFirstWorkflow(ctx, task, project) bool`: nếu `project.ExecutionProviders` rỗng, giữ nguyên hành vi cũ (`ResolveEngine(...) == "cli"`, byte-identical — không đổi hành vi project cũ). Nếu không rỗng, gọi `ResolveExecutionProvider` và trả `resolved.Type == "cli"`.
- `worker.go:309` gọi hàm mới này thay vì `cliengine.ResolveEngine(...)` trực tiếp.

### Issue 3: `TaskService.validateTaskEngineOverride` check `ExecutionProviders` trước
- Trước khi fallback về check `CLIEngineConfig.Command`, nếu `project.ExecutionProviders` không rỗng: hợp lệ khi có ít nhất 1 entry `type=="cli" && enabled==true` trong list (không cần biết credential có khả dụng ngay lúc validate — đúng tinh thần "Router tự chọn candidate lúc Task chạy", validate ở đây chỉ chặn trường hợp rõ ràng vô nghĩa).
- Logic fallback giống hệt `legacyResolveExecutionProvider`'s "empty providers → dùng field cũ" — tái dùng, không viết lại.

## Capabilities

### Modified Capabilities
- `cliStepRunner.resolveConfig` — đọc qua `ResolveExecutionProvider` thay vì `CLIEngineConfig` thẳng.
- `worker.go`'s workflow-shape selection — nhất quán với Router thay vì chỉ field cũ.
- `TaskService.validateTaskEngineOverride` — nhận diện `ExecutionProviders` là nguồn cấu hình hợp lệ.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Backend orchestrator | `server/internal/orchestrator/cli_spec_step.go`, `server/internal/orchestrator/worker.go` |
| Backend service | `server/internal/service/task.go` |
| Backend tests | `server/internal/orchestrator/cli_spec_step_test.go`, `server/internal/orchestrator/worker_test.go` (nếu có), `server/internal/service/task_test.go` |
