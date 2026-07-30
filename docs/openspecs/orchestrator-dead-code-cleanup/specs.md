# Specs: Orchestrator Dead Code Cleanup

## Removed Requirements

### REQ-R01: `engine.apiNativeEngine` bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` chạy sau khi xoá `api_native.go`/`api_native_test.go`
- THEN build thành công — không có import nào còn tham chiếu `NewAPINativeEngine`/`apiNativeEngine` (đã verify 0 call site sản xuất trước khi xoá)

### REQ-R02: `promptAssemblerAdapter` bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` chạy sau khi xoá `promptAssemblerAdapter`
- THEN build thành công, `WithPrompts` vẫn nhận `*prompts.PromptAssembler` trực tiếp như hôm nay

### REQ-R03: `readAnalyzeFile`/`grepAnalyzeFiles` bị xoá (hoặc giữ + extract helper — xem Task 1.3)
> ✅ Status: Fully Implemented

**Scenario: Xoá**
- WHEN `go build ./...` + `go test ./internal/orchestrator/steps/...` chạy sau khi xoá 2 method + test tương ứng
- THEN build và test đều pass, `executeAnalyzeTool`'s dispatch (dùng `s.registry.Execute`) không đổi hành vi

**Scenario: Giữ lại (nếu team quyết định cần dispatch surface tương lai)**
- WHEN quyết định giữ lại thay vì xoá
- THEN phần logic "resolve workspace roots" dùng chung giữa `readAnalyzeFile`/`grepAnalyzeFiles`/`listAnalyzeFiles` được extract thành 1 helper — không còn 3 bản copy-paste
