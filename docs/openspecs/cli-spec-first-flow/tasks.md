# Tasks: CLI Spec-First Flow

> Prerequisite: `pluggable-execution-engine/` hoàn thành (engine interface + cliEngine hoạt động).

## 1. Workflow definition (REQ-001, REQ-M01)

- [x] 1.1 `server/internal/workflow/step.go`: thêm step IDs `cli_analyze`, `cli_spec`, `cli_implement`, `cli_mr` + definition `cli_spec_first`
- [x] 1.2 `BuildWorkflow` nhận engine; chọn definition; call-sites cập nhật
- [x] 1.3 Tests: build đúng definition theo engine; api_native snapshot không đổi

## 2. Prompt templates

- [x] 2.1 `server/internal/prompts/steps/cli_analyze.md`
- [x] 2.2 `cli_spec.md` (nhúng 4-file OpenSpec convention + chỉ dẫn bắt buộc về frontmatter `type: documentation` cho task docs-only — thiếu câu này bypass qua frontmatter vô hiệu)
- [x] 2.3 `cli_implement.md`
- [x] 2.4 Loading + tests — **deviation**: dùng `LoadStepPrompt` (file-load nhẹ, không qua `PromptAssembler`) thay vì wiring vào Assembler; xem `docs/implementation/cli-spec-first-flow-notes.md`

## 3. Steps (REQ-002, REQ-003, REQ-005)

- [x] 3.1 `steps/cli_analyze.go`: spawn qua engine → validate `.autocode/analysis.md` → lưu `task.Analysis`
- [x] 3.2 `steps/cli_spec.go`: spawn → validate 4 files + ≥1 checkbox → fail message liệt kê file thiếu
- [x] 3.3 `steps/cli_implement.go`: spawn → validate diff ngoài `docs/openspecs/` (+ docs-only bypass qua label/frontmatter) → checkpoint → progress count
- [x] 3.3b Helper `ParseCheckboxes`: strip fenced code blocks TRƯỚC rồi mới regex `(?im)^\s*[-*]\s*\[([ xX])\]` + tests: biến thể format (`- [X]`, `* [x]`, thừa space) và checkbox trong code block KHÔNG được đếm
- [x] 3.4 `cli_mr`: wire reuse `steps/pr.go` (struct embedding `CLIMRStep{ *PRStep }`), PR description nhúng proposal Why/What
- [x] 3.5 Unit tests từng step với mock CLIStepRunner (missing files, spec-only diff, happy path)

## 4. Approval gate (REQ-004)

- [x] 4.1 Khảo sát cơ chế pause/boundary-resolution hiện có; **quyết định**: tái dùng state machine `TaskStatusSpecReview`/`TaskSpecStatus{PendingReview,Approved,ChangesRequested}` sẵn có (không thêm `awaiting_spec_approval`)
- [x] 4.2 Worker: `CLISpecStep` pause sau cli_spec khi `project.DefaultAutonomy != "autonomous"`; resume-guard skip re-spawn khi `SpecStatus == approved`
- [x] 4.3 API `POST /tasks/{id}/spec-review` (approve / request_changes + comment, giới hạn `MaxReviewFixCycles` qua `CheckSpecReviewLoopLimit`)
- [x] 4.4 Re-dispatch cli_spec với reviewer feedback trong prompt (`## Reviewer feedback` section khi `SpecStatus == changes_requested`)
- [x] 4.5 Tests: gate bật/tắt theo autonomy (`TestCLISpecStep_SupervisedPauses`, `TestCLISpecStep_HappyPath`), resume-after-approval (`TestCLISpecStep_ResumeAfterApprovalSkipsRunner`)

## 5. Spec read API + Web UI (REQ-006, REQ-007)

- [x] 5.1 `GET /tasks/{id}/spec` đọc từ worktree (`Orchestrator.GetTaskSpec`), trả 4 docs + progress
- [x] 5.2 Web: `CLISpecPanel.tsx` — spec panel riêng cho CLI flow (render markdown, checkbox progress bar), fetch qua `useAuthedSWR`
- [x] 5.3 Approve/Request-changes controls: `CLISpecReviewControls.tsx` (mô hình theo `BoundaryResolutionControls.tsx`, gated bằng pause reason string thay vì regex boundary)
- [x] 5.4 Timeline: `CheckpointsPanel.tsx` map step ID → tên thân thiện (`STEP_LABELS`)
- [x] 5.5 Web build + lint pass (`next build`, `eslint` — clean)

## 6. E2E & docs

- [x] 6.1 Integration test: task engine=cli chạy đủ 4 step với fake CLI engine runner tạo files thật trong worktree tạm (`steps/cli_spec_first_integration_test.go`)
- [x] 6.2 Cập nhật `ARCHITECTURE.md` + roadmap status
- [x] 6.3 Update status icons trong `specs.md`

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
