# Proposal: Infra/Tooling Dead Code Cleanup

## Why

Audit `internal/sandbox` (trừ network bug — đã tách sang [`sandbox-network-isolation-fix/`](../sandbox-network-isolation-fix/)), `internal/gitops`, `internal/governance`, `internal/policy`, `internal/workflow`, `internal/tool`, `internal/evals`, `internal/database`, `pkg/paths` phát hiện: 1 package hoàn toàn không wiring (`evals`), 1 adapter chết theo tính năng đã đổi cách gọi (`SkillExecutorAdapter`), logic trùng lặp 2-3 lần cho cùng 1 việc (parse Go build error, validate DAG), 1 field cấu hình không bao giờ ảnh hưởng hành vi (`governance.Extends`), và vài helper nhỏ 0 caller.

## What Changes

### Issue 1: Package `internal/evals` không wiring
- `datasets.go`/`evaluator.go` (`Evaluator`, `MemoryDatasetStore`, `KeywordJudge`...) — 0 importer ngoài chính package, không `cmd/` entrypoint, không handler route.
- Xoá cả package, hoặc nếu là groundwork có chủ đích cho 1 tính năng eval-harness sắp làm — giữ nhưng ghi rõ trong `ARCHITECTURE.md`/README là "chưa wiring, chờ OpenSpec riêng" thay vì để mồ côi không giải thích.

### Issue 2: `SkillExecutorAdapter` + `internal/orchestrator/skills` chết
- `internal/tool/adapter.go` (comment tự nhận "adapts the new Registry to the legacy SkillExecutor.Execute signature") + `internal/orchestrator/skills/executor.go` (`SkillCall`/`SkillResult`) — 0 caller sản xuất.
- Xoá `adapter.go`+`adapter_test.go`, xoá luôn package `internal/orchestrator/skills` nếu trống sau đó.

### Issue 3: Gộp logic parse Go build error (trùng lặp 2 lần)
- `internal/tool/verify/compile_check.go` (`compileErrorRegex`) và `internal/tool/tools/run_build.go` (`goCompilerErrorRegex`) — cùng 1 regex, cùng 1 cách parse ra `[]tool.Diagnostic`.
- Extract 1 hàm `ParseGoBuildOutput([]byte) []tool.Diagnostic` dùng chung trong `internal/tool`.

### Issue 4: Gộp/thống nhất 3 bản `ValidateDAG` (trùng tên, khác chữ ký)
- `internal/workflow/graph.go`, `internal/governance/dag.go`, `internal/policy/scheduler_policy.go` — 3 hàm cùng tên `ValidateDAG`, mỗi cái tự viết cycle-detector cho 1 shape dữ liệu khác nhau.
- Extract 1 `pkg/graph` package với cycle-detector generic (nhận accessor `id()`/`deps()`), 3 package trên gọi vào thay vì tự viết — giảm ~100 dòng trùng lặp, xoá nhầm lẫn tên gọi giữa 3 domain khác nhau (workflow/governance/policy).

### Issue 5: `governance.Pipeline.Extends` không thật sự làm gì
- Field được document là "patch-style overrides against a preset" (`api_native`/`cli_spec_first`), nhưng chỉ được đọc như 1 boolean (`!= ""`) ở `isFullCustomGraph` — chọn preset nào cũng cho hành vi giống hệt nhau. Tên preset (`api_native`) còn trùng thuật ngữ mà `cli-execution-provider-routing` vừa thay thế ở tầng khác (`Project.ExecutionEngine`), dễ gây nhầm lẫn 2 khái niệm không liên quan.
- **Cần quyết định hướng trước khi code**: (a) wiring `Extends` thành thứ thật (mỗi preset thật sự áp 1 step-shape template khác nhau), hay (b) đơn giản hoá — bỏ khái niệm "preset" nếu không ai dùng tới sự khác biệt giữa `api_native`/`cli_spec_first` presets trong thực tế, chỉ giữ field boolean thật (`IsPatchMode` hay tương tự) đúng với những gì code thật sự làm.

### Issue 6: Dead helpers nhỏ
- `internal/gitops/adapter.go:60-62` `GitOpsAdapter.CloneRepo` — passthrough 0 caller (mọi nơi dùng `CloneForTask` hoặc `GitProvider.CloneRepo` thẳng). Xoá.
- `internal/database/database.go:24` `Connect` — 0 caller, cả 2 entrypoint dùng `ConnectWithPool` thẳng. Xoá.
- `pkg/paths/workspace.go:139,151` `RepoRelativeToWorkspace`, `IsWorkspaceInternalPath` — 0 caller. Xoá.
- `pkg/paths/types.go:9` `Path` interface — không dùng như kiểu tham số/return ở đâu. Xoá (hoặc giữ nếu có kế hoạch dùng sớm — mức ưu tiên thấp nhất trong OpenSpec này).

## Capabilities

### Removed Capabilities
- `internal/evals` (trừ khi Task 1.1 quyết định giữ).
- `SkillExecutorAdapter`, `internal/orchestrator/skills`.
- `GitOpsAdapter.CloneRepo`, `database.Connect`, 2 helper trong `pkg/paths`.

### Modified Capabilities
- `ValidateDAG` (3 chỗ) tái dùng 1 cycle-detector chung thay vì 3 bản độc lập.
- `governance.Pipeline.Extends` — hành vi thật hoặc bị đơn giản hoá (tuỳ quyết định Task 5.1).

## Impact

| Area | Files Affected |
|------|----------------|
| Backend infra | `server/internal/evals/` (xoá hoặc giữ), `server/internal/tool/adapter.go`, `server/internal/orchestrator/skills/`, `server/internal/tool/verify/compile_check.go`, `server/internal/tool/tools/run_build.go`, `server/internal/gitops/adapter.go`, `server/internal/database/database.go`, `server/pkg/paths/workspace.go`, `server/pkg/paths/types.go` |
| Backend graph logic | `server/pkg/graph/` (mới), `server/internal/workflow/graph.go`, `server/internal/governance/dag.go`, `server/internal/policy/scheduler_policy.go` |
| Backend governance | `server/internal/governance/config.go`, `server/internal/governance/validate.go`, `server/internal/governance/presets.go` (tuỳ Task 5.1) |
