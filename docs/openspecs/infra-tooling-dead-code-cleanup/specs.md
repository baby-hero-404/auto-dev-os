# Specs: Infra/Tooling Dead Code Cleanup

## Removed Requirements

### REQ-R01: `internal/evals` bị xoá (hoặc chính thức document là chưa wiring)
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` sau khi xoá `internal/evals`
- THEN build thành công, không package nào import nó (đã verify)

### REQ-R02: `SkillExecutorAdapter`/`internal/orchestrator/skills` bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` sau khi xoá `adapter.go`+`adapter_test.go`+package `skills`
- THEN build thành công, tool-dispatch path thật (qua `Registry`) không đổi hành vi

### REQ-R03: Dead helpers nhỏ bị xoá
> ✅ Status: Partially Implemented — 2/4 xoá, 2/4 revert (false positive)

**Scenario (đã xoá — đúng):**
- WHEN `go build ./...` sau khi xoá `database.Connect`, `RepoRelativeToWorkspace`, `IsWorkspaceInternalPath`
- THEN build thành công, `ConnectWithPool`/`WorkspaceToRepoRelative` (đang dùng thật) không đổi hành vi

**Scenario (KHÔNG xoá — audit gốc sai):**
- `GitOpsAdapter.CloneRepo`: satisfy `orchestrator.GitOpsClient` interface (`WithGitOpsClient` cần method này) — build fail ngay khi thử xoá. Audit chỉ grep tên gọi trực tiếp, bỏ sót interface satisfaction.
- `paths.Path` interface: dùng không-qualify trong chính package (`fs.go`'s `OSFileSystem.Exists(p Path)`, `testing.go`'s `InMemoryFileSystem.Exists(p Path)`) — audit chỉ grep `paths\.Path\b` (qualify từ ngoài), bỏ sót usage trong-package.
- Cả 2 đều bị bắt ngay lập tức bởi `go build` (không phải chạy sót qua CI) — quy trình "build sau mỗi lần xoá" trong `design.md` hoạt động đúng như thiết kế.

## Modified Requirements

### REQ-M01: Parse Go build error dùng chung 1 hàm
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `CompileCheckHook` (verify pipeline) và `RunBuildTool` (agent tool) cùng nhận 1 output `go build` giống nhau (có lỗi compile)
- THEN cả 2 trả về `[]tool.Diagnostic` giống hệt nhau (cùng file/line/message) — không còn 2 regex độc lập có thể lệch nhau theo thời gian

### REQ-M02: `ValidateDAG` (workflow/governance/policy) dùng chung cycle-detector
> 🚫 Status: Cancelled — không phải duplication thuần

**Ghi chú:** Sau khi đọc kỹ cả 3 implementation, phát hiện chúng xử lý khác nhau thật ở edge case "dependency trỏ tới node không tồn tại" (`workflow` → false cycle, `policy` → skip có chủ đích, `governance` → violation riêng + dừng check khác), và chỉ `governance` có test cho edge case này. Unify an toàn đòi hỏi thêm 1 tham số "strategy" cho từng domain — tăng phức tạp thay vì giảm. Huỷ, để lại làm khuyến nghị cho task riêng có thời gian test đầy đủ hơn.

### REQ-M03: `governance.Pipeline.Extends` — quyết định 1 trong 2 hướng
> 🚫 Status: Cancelled — user chọn giữ nguyên, không đụng vào

**Ghi chú:** Cả option (a) wiring thật lẫn (b) đơn giản hoá đều là thay đổi hành vi/API cần spec riêng, không phải cleanup dead-code thuần — user xác nhận để nguyên.
