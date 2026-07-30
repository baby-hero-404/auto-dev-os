# Design: Orchestrator Dead Code Cleanup

## Approach

Đây là dọn dead code thuần tuý — không có kiến trúc mới. Quy trình cho mỗi item, bắt buộc theo thứ tự:

1. **Re-verify ngay trước khi xoá**: `grep -rn "<TênSymbol>" server/` (bao gồm `_test.go`) — code có thể đã đổi từ lúc audit chạy tới lúc thực thi task. Nếu xuất hiện call site mới ngoài audit đã ghi nhận, dừng lại, không xoá, note lại lý do.
2. Xoá symbol + file/test liên quan.
3. `go build ./...` + `go vet ./...` — bắt import chết/unused ngay lập tức.
4. `go test ./...` toàn bộ (không chỉ package bị đụng) — dead-code removal về lý thuyết không đổi hành vi, nhưng 1 lần chạy full suite là chi phí rẻ để chắc chắn.

## Item-by-item

### `engine/api_native.go`
```bash
grep -rn "NewAPINativeEngine\|apiNativeEngine\b" server/
```
Kỳ vọng: chỉ còn match trong chính file sắp xoá. Xoá `api_native.go` + `api_native_test.go` bằng `git rm`.

### `service_adapters.go` — `promptAssemblerAdapter`
```bash
grep -rn "promptAssemblerAdapter" server/
```
Xoá type + method. Không đổi `WithPrompts`/`orchestrator.go` (đã nhận đúng interface).

### `steps/analyze_tools.go` — `readAnalyzeFile`/`grepAnalyzeFiles`
```bash
grep -rn "\.readAnalyzeFile(\|\.grepAnalyzeFiles(" server/
```
Nếu chỉ còn match trong `analyze_test.go`: xoá cả 2 method + test case tương ứng trong `analyze_test.go`.
Nếu team (qua review OpenSpec này) muốn giữ làm dispatch surface tương lai thay vì xoá: extract phần "resolve workspace roots từ `TaskWorkspace.Repos`" (lặp lại 3 lần trong `readAnalyzeFile`/`grepAnalyzeFiles`/`listAnalyzeFiles`) thành 1 helper `resolveAnalyzeWorkspaceRoots(...)`, cả 3 method gọi chung.

## Trade-offs

- **Không gộp chung 1 PR với các OpenSpec cleanup khác** — mỗi OpenSpec là 1 đơn vị review độc lập theo quyết định "nhiều OpenSpec nhỏ theo subsystem", dù về mặt kỹ thuật có thể gộp an toàn (tất cả đều pure deletion).
