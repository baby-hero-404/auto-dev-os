# Proposal: Service/Repository/Models Dead Code Cleanup

## Why

Audit `internal/service`, `internal/repository`, `pkg/models` phát hiện nhiều method/field không còn call site — phần lớn là phần ghi (write path) của các tính năng đã bị rút gọn/thay đổi hướng (skill suggestion không còn tạo qua UI, knowledge-edge chỉ còn đọc không còn ghi, audit IP-tracking chưa từng được wiring) — cùng 1 chỗ inconsistency về cách wrap lỗi giữa các repository nên chuẩn hoá.

Gap về `TaskService.validateTaskEngineOverride` không check `ExecutionProviders` (audit finding #8) đã tách sang [`execution-provider-routing-adoption-gaps/`](../execution-provider-routing-adoption-gaps/) vì nó là bug đang active, không phải dead code — không lặp lại ở đây.

## What Changes

### Issue 1: Dead repository methods (atomic-claim đã thay thế bản non-atomic)
- `internal/repository/agent.go`: `FindAvailableByRole`, `FindByRole`, `FindAnyAvailable` — 0 caller, đã bị thay bởi `ClaimAvailableByRole`/`ClaimAnyAvailable` (dùng `SELECT ... FOR UPDATE SKIP LOCKED`). Xoá cả 3.
- `internal/repository/workflow.go:237` `ResetAllRunningJobs` — chỉ có trong interface + mock, 0 real caller (`ResetStuckJobs` là bản thật đang dùng ở `queue.go:73`). Xoá method + interface entry + 2 mock stub, **hoặc** nếu đây là 1 startup-reset dự định nhưng bị bỏ sót khi wiring, note lại thành gap cần điền (không tự ý xoá nếu nghi ngờ là bug thiếu, không phải dead code — xem Task 1.2).

### Issue 2: Knowledge-edge write path chết
- `internal/service/memory.go:240` `CreateEdge`, `internal/repository/knowledge_edge.go:19,48` (`Create`/`Delete`) — 0 caller thật, chỉ đọc (`GetEdgesByMemory`/`ListBySource`/`ListByTarget`) đang sống. Cần xác nhận: đây là tính năng dự kiến (authoring UI cho knowledge graph) chưa ship, hay hoàn toàn không còn trong roadmap — quyết định khác nhau (giữ chờ UI vs xoá hẳn).

### Issue 3: Audit IP-tracking chết
- `internal/service/audit.go`: `WithUserID`/`WithAgentID`/`WithIPAddress` chỉ dùng trong test; `pkg/models/audit.go` `AuditLog.IPAddress` không bao giờ được ghi bởi code thật (không middleware nào capture IP). Quyết định: wiring thật (thêm middleware capture IP cho audit log) hay xoá field+options.

### Issue 4: `LearningService.applySkillSuggestion` — plumbing chết theo tính năng đã gỡ
- `internal/service/learning.go`: field `skills *SkillService` + `SetSkillService` được wiring từ `cmd/api/main.go:193`, nhưng `applySkillSuggestion` đã bị sửa để luôn trả lỗi cố định ("skill creation is no longer supported on the UI...") — không bao giờ đụng tới `s.skills`. Đây đúng pattern "tính năng bị rút, plumbing bị bỏ quên" giống `ExecutionEngine`/`CLIEngineConfig` nhưng **chưa được document** là legacy — xoá field + setter + wiring call, giữ nguyên message lỗi cố định.

### Issue 5: Constants/structs chết nhỏ
- `pkg/models/workflow.go:27` `WorkflowStepSandbox` — không dùng ở đâu, xoá.
- `pkg/models/agent.go:96-100` `AgentSkill` struct — 0 reference Go dù bảng `agent_skills` có thật trong migration. **Không tự xoá bảng** — chỉ flag cho product xác nhận đây là tính năng chưa implement hay đã bỏ, việc drop bảng (nếu cần) nằm ngoài phạm vi OpenSpec cleanup thuần code.
- `pkg/models/pull_request.go:14-16` `PRStatusApproved`/`PRStatusRejected`/`PRStatusMerged` — không bao giờ được gán (PR state thật sự sống ở `Task.Status`). Độ tin cậy thấp hơn các item khác — cần 1 câu hỏi nhanh cho chủ UI PR review trước khi xoá.

### Issue 6: Chuẩn hoá error-wrapping giữa các repository
- `mapError` (`repository/errors.go`) được dùng ở 12/~30 file repo, số còn lại (`analytics*.go`, `attestation.go`, `audit.go`, `knowledge_edge.go`, `learned_skill.go`, `secrets.go`) trả raw gorm error — `handler/response.go` phải check cả 3 kiểu lỗi (`service.ErrNotFound`, `repository.ErrNotFound`, `gorm.ErrRecordNotFound`) để bù. Chuẩn hoá 1 hướng: dùng `mapError` cho toàn bộ, hoặc xác nhận raw-gorm là convention chính thức và xoá `mapError` khỏi các file đã dùng nó (không giữ 2 convention song song không lý do).

## Capabilities

### Removed Capabilities
- `agent.FindAvailableByRole`/`FindByRole`/`FindAnyAvailable`.
- `LearningService.skills`/`SetSkillService` (plumbing, không phải chức năng — chức năng đã bị rút từ trước).
- `WorkflowStepSandbox` constant.

### Modified Capabilities
- Error-wrapping convention thống nhất giữa các repository (hướng cụ thể do Task 6.1 quyết định).

## Impact

| Area | Files Affected |
|------|----------------|
| Backend repository | `server/internal/repository/agent.go`, `server/internal/repository/workflow.go`, `server/internal/repository/knowledge_edge.go`, và tuỳ Issue 6, các file repo còn lại |
| Backend service | `server/internal/service/memory.go`, `server/internal/service/audit.go`, `server/internal/service/learning.go` |
| Backend models | `server/pkg/models/workflow.go`, `server/pkg/models/agent.go`, `server/pkg/models/pull_request.go`, `server/pkg/models/audit.go` |
| Backend entrypoint | `server/cmd/api/main.go` (bỏ `SetSkillService` wiring) |
| Backend mocks/tests | `server/internal/orchestrator/mock_test.go`, `server/internal/handler/pr_test.go` (2 mock `ResetAllRunningJobs`) |
