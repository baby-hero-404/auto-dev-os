# Tasks: Orchestrator Dead Code Cleanup

## P1 — dead code removal

### Task 1.1: Xoá `engine.apiNativeEngine`
- [x] Re-verify `grep -rn "NewAPINativeEngine\|apiNativeEngine\b" server/`.
- [x] `git rm server/internal/orchestrator/engine/api_native.go server/internal/orchestrator/engine/api_native_test.go`.
- [x] `go build ./...`, `go vet ./...`.
- Satisfies: REQ-R01

### Task 1.2: Xoá `promptAssemblerAdapter`
- [x] Re-verify `grep -rn "promptAssemblerAdapter" server/`.
- [x] Xoá type + method trong `service_adapters.go`.
- [x] `go build ./...`.
- Satisfies: REQ-R02

### Task 1.3: `readAnalyzeFile`/`grepAnalyzeFiles` — quyết định xoá hay giữ+extract
- [x] Re-verify `grep -rn "\.readAnalyzeFile(\|\.grepAnalyzeFiles(" server/`.
- [x] Nếu xoá: xoá 2 method + test case tương ứng.
- [x] Nếu giữ: extract helper dùng chung cho cả 3 method (design.md).
- [x] `go test ./internal/orchestrator/steps/...`.
- Satisfies: REQ-R03

### Task 1.4: Full regression sau khi xoá cả 3
- [x] `go build ./...`
- [x] `go test ./...` (toàn bộ server, không chỉ package bị đụng)
- Satisfies: REQ-R01, REQ-R02, REQ-R03 (verification)

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-R01 | 1.1 |
| REQ-R02 | 1.2 |
| REQ-R03 | 1.3 |
