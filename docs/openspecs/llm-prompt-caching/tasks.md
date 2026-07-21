# Tasks: Anthropic Prompt Caching

- [ ] 1.1 Audit cache partition: xác định vị trí hiện tại của repomap/diffs/task-instruction trong assembly (`internal/prompts/`); di chuyển mọi nội dung đổi-trong-job xuống messages; test 2 lần assemble cùng job → prefix identical bytes
- [ ] 1.2 `server/pkg/llm/anthropic.go`: system → array-of-blocks (nếu đang string), thêm `cache_control` vào block cuối system + tool cuối
- [ ] 1.3 Parse `cache_creation_input_tokens` / `cache_read_input_tokens` vào Usage struct + usage log
- [ ] 1.4 Guard: field `cache_control` chỉ render cho Anthropic provider (REQ-004)
- [ ] 1.5 Unit tests: request body snapshot (đúng vị trí breakpoints, ≤4), non-Anthropic không có field
- [ ] 1.6 Integration smoke (nếu có key test): vòng 2 của tool-loop có cache_read > 0; ghi kết quả vào specs.md status
