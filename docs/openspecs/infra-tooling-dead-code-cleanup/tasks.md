# Tasks: Infra/Tooling Dead Code Cleanup

## P1 — xoá thẳng, đã verify

### Task 1.1: `internal/evals`
- [x] Re-verify 0 importer ngoài package.
- [x] Xoá package (`server/internal/evals/`).
- Satisfies: REQ-R01

### Task 1.2: `SkillExecutorAdapter` + `internal/orchestrator/skills`
- [x] Re-verify 0 caller.
- [x] Xoá `adapter.go`+`adapter_test.go`; package `skills` trống sau đó, tự biến mất.
- Satisfies: REQ-R02

### Task 1.3: Dead helpers nhỏ — 2/4 xác nhận chết, 2/4 hoá ra KHÔNG chết (false positive từ audit)
- [x] `GitOpsAdapter.CloneRepo`: **KHÔNG xoá** — build fail ngay khi thử xoá vì method này satisfy interface `orchestrator.GitOpsClient` (`WithGitOpsClient` yêu cầu `CloneRepo`). Audit gốc chỉ grep tên gọi trực tiếp, bỏ sót interface satisfaction — đúng loại lỗi mà `design.md` đã cảnh báo trước ("check for interface satisfaction"). Giữ nguyên.
- [x] `database.Connect`: xoá — xác nhận là hàm package-level (không phải method, không thể satisfy interface), 0 caller thật.
- [x] `pkg/paths.RepoRelativeToWorkspace`/`IsWorkspaceInternalPath`: xoá — xác nhận 0 caller.
- [x] `paths.Path` interface: **KHÔNG xoá** — lần đầu grep chỉ tìm `paths\.Path\b` (cross-package), bỏ sót 3 chỗ dùng `Path` không-qualify NGAY TRONG package (`fs.go`, `testing.go` — `OSFileSystem.Exists(p Path)`, `InMemoryFileSystem.Exists(p Path)`). Build fail ngay khi xoá, revert. Bài học thứ 2 trong cùng 1 task: grep phải quét toàn bộ package bằng tên không-qualify, không chỉ tên có qualify từ ngoài.
- Satisfies: REQ-R03 (thu hẹp còn `database.Connect` + 2 helper trong `pkg/paths/workspace.go`)

### Task 1.4: Full regression
- [x] `go build ./...`, `go vet ./...`, `go test ./...` sau Task 1.1-1.3 — tất cả pass.
- Satisfies: REQ-R01, REQ-R02, REQ-R03 (verification)

## P2 — refactor trùng lặp

### Task 2.1: `ParseGoBuildOutput` dùng chung
- [x] Extract `internal/tool/build_output.go` (`ParseGoBuildOutput(stdout, stderr string) []Diagnostic`), cập nhật `compile_check.go`+`run_build.go` gọi vào — cả 2 fallback-message logic (khác nhau nhẹ giữa 2 caller) giữ nguyên tại chỗ, chỉ phần regex+parse-loop dùng chung.
- [x] `go test ./internal/tool/...` — `TestCompileCheckHook`, `TestRunBuildTool` pass, diagnostic output không đổi.
- Satisfies: REQ-M01

### Task 2.2: `pkg/graph` dùng chung cho 3 `ValidateDAG` — **không làm, quyết định có chủ đích**
- [x] Đọc kỹ cả 3 implementation (`workflow/graph.go` Kahn's-algorithm topo-sort, `governance/dag.go` DFS 3-màu + 4 check cấu trúc khác, `policy/scheduler_policy.go` DFS 3-màu + cost threshold) trước khi động code.
- [x] Phát hiện: đây **không phải duplication thuần** — 3 implementation có hành vi khác nhau thật ở edge case "dependency trỏ tới node không tồn tại": `workflow` coi là cycle (do thuật toán Kahn's không giảm in-degree được), `policy` **cố tình bỏ qua** (`if !allNodes[neighbor] { continue }`), `governance` báo lỗi "unresolved dependency" riêng và dừng các check khác. Chỉ `governance` có test cho edge case này (`TestValidateDAG_UnresolvedDependency`); `policy` không có test nào cho hành vi skip của nó.
- [x] Kết luận: unify thành 1 `graph.DetectCycle` chung đòi hỏi hoặc đổi hành vi observable của ít nhất 1 package, hoặc thêm tham số "strategy" để giữ nguyên 3 cách xử lý khác nhau — cả 2 hướng đều làm tăng độ phức tạp thay vì giảm, và rủi ro cao hơn lợi ích thu được (giảm ~100 dòng trùng lặp bề mặt, nhưng lõi cycle-detection lại không thực sự trùng nhau về hành vi). **Không làm** — để lại làm khuyến nghị cho 1 task riêng, có thời gian test kỹ hơn từng edge case của cả 3 domain, không bundle vào đợt dọn dead-code này.
- Satisfies: REQ-M02 — **huỷ** (xem specs.md)

## P2 — quyết định trước khi code

### Task 3.1: `governance.Pipeline.Extends`
- [x] Hỏi user: option (a) wiring thật hay (b) đơn giản hoá?
- [x] **Quyết định: để nguyên, không đụng vào** — cả 2 hướng đều là thay đổi hành vi/API cần spec riêng, không phải dead-code cleanup thuần.
- Satisfies: REQ-M03 — **huỷ** (giữ nguyên field, không sửa)

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-R01 | 1.1 |
| REQ-R02 | 1.2 |
| REQ-R03 | 1.3 (thu hẹp phạm vi) |
| REQ-M01 | 2.1 |
| REQ-M02 | 2.2 — huỷ, không làm |
| REQ-M03 | 3.1 — huỷ, giữ nguyên |
