# Tasks: CLI Spec-First Flow

> Prerequisite: `pluggable-execution-engine/` hoàn thành (engine interface + cliEngine hoạt động).

## 1. Workflow definition (REQ-001, REQ-M01)

- [ ] 1.1 `server/internal/workflow/step.go`: thêm step IDs `cli_analyze`, `cli_spec`, `cli_implement`, `cli_mr` + definition `cli_spec_first`
- [ ] 1.2 `BuildWorkflow` nhận engine; chọn definition; call-sites cập nhật
- [ ] 1.3 Tests: build đúng definition theo engine; api_native snapshot không đổi

## 2. Prompt templates

- [ ] 2.1 `server/internal/prompts/steps/cli_analyze.md`
- [ ] 2.2 `cli_spec.md` (nhúng 4-file OpenSpec convention + chỉ dẫn bắt buộc về frontmatter `type: documentation` cho task docs-only — thiếu câu này bypass qua frontmatter vô hiệu)
- [ ] 2.3 `cli_implement.md`
- [ ] 2.4 Assembler wiring + tests (theo pattern `prompts/assembler_test.go`)

## 3. Steps (REQ-002, REQ-003, REQ-005)

- [ ] 3.1 `steps/cli_analyze.go`: spawn qua engine → validate `.autocode/analysis.md` → lưu `task.Analysis`
- [ ] 3.2 `steps/cli_spec.go`: spawn → validate 4 files + ≥1 checkbox → fail message liệt kê file thiếu
- [ ] 3.3 `steps/cli_implement.go`: spawn → validate diff ngoài `docs/openspecs/` (+ docs-only bypass qua label/frontmatter) → checkpoint → progress count
- [ ] 3.3b Helper `parseCheckboxes`: strip fenced code blocks TRƯỚC rồi mới regex `(?im)^\s*[-*]\s*\[([ xX])\]` + tests: biến thể format (`- [X]`, `* [x]`, thừa space) và checkbox trong code block KHÔNG được đếm
- [ ] 3.4 `cli_mr`: wire reuse `steps/pr.go`, PR description nhúng proposal Why/What
- [ ] 3.5 Unit tests từng step với MockEngine (missing files, spec-only diff, happy path)

## 4. Approval gate (REQ-004)

- [ ] 4.1 Khảo sát cơ chế pause/boundary-resolution hiện có; quyết định tái dùng hay thêm trạng thái `awaiting_spec_approval`
- [ ] 4.2 Worker: pause sau cli_spec khi supervised; dispatch tiếp khi approve
- [ ] 4.3 API `POST /tasks/{id}/spec-review` (approve / request_changes + comment, giới hạn `MaxReviewFixCycles`)
- [ ] 4.4 Re-dispatch cli_spec với reviewer feedback trong prompt
- [ ] 4.5 Tests: gate bật/tắt theo autonomy, vòng lặp request_changes chạm max

## 5. Spec read API + Web UI (REQ-006, REQ-007)

- [ ] 5.1 `GET /tasks/{id}/spec` đọc từ worktree, trả 4 docs + progress
- [ ] 5.2 Web: Spec panel trong task detail (render markdown, checkbox progress bar)
- [ ] 5.3 Approve/Request-changes controls (reuse pattern `BoundaryResolutionControls.tsx`)
- [ ] 5.4 Timeline hiển thị 4 step mới với tên thân thiện
- [ ] 5.5 Web build + lint pass

## 6. E2E & docs

- [ ] 6.1 Integration test: task engine=cli chạy đủ 4 step với MockEngine tạo files thật trong worktree tạm
- [ ] 6.2 Cập nhật `ARCHITECTURE.md` + roadmap status
- [ ] 6.3 Update status icons trong `specs.md`
