# Proposal: CLI Platform Knowledge Injection (v2 — Context Materializer)

## Why

CLI spec-first steps (`cli_analyze`, `cli_spec`, `cli_implement`) build instruction cực mỏng — chỉ nối `step_prompt.md + Task Title + Description`. Platform có knowledge mà container CLI không tự thấy được (không mount `.data/`, không có network tới Postgres): Skills page skills, `learned_skills` (bài học từ task đã merge trước), `task.Analysis.TaskRules`.

Hai vòng lặp trước của spec này đã bị loại vì sai boundary:
- **v1** (`AddedRequirements` cũ REQ-001..004): dump full text top-5 skill thẳng vào instruction string. Sai vì platform tự quyết định thay agent cái gì quan trọng, và có bug (relevance-scoring không thực sự chạy, thiếu 3/4 file test mock).
- **v1.5** (mount toàn bộ `.data/projects/{id}/skills/` vào `.autocode/skills/`, chỉ trỏ 1 câu trong instruction): đúng hướng "để agent tự khám phá qua filesystem" nhưng **chỉ giải quyết skills**, không tổng quát cho `learned_skills`/`task_rules`, và mount "toàn bộ" không scale nếu project có hàng trăm/nghìn skill (agent phải tự đọc hết thư mục để biết cái nào liên quan — không có bất kỳ gợi ý relevance nào).

**v2 (hướng này)** giữ đúng insight của v1.5 (filesystem là source of truth, agent tự quyết đọc gì) nhưng tổng quát hoá thành 1 **Context Materializer**: platform resolve trước phần "liên quan" (relevant) bằng chính scoring logic đã có (`resolveSkills`), rồi vật chất hoá thành file thật trong `.autocode/context/` — dùng lại đúng ephemeral `.autocode/` convention đã tồn tại (tự tạo/tự dọn quanh mỗi lần chạy CLI, không bao giờ commit vào git). Instruction chỉ cần 1 đoạn ngắn trỏ tới `$AUTOCODE_CONTEXT_DIR` — agent tự quyết đọc gì, lúc nào, sâu tới đâu.

> Platform chỉ đảm bảo: "Knowledge tồn tại trong environment." Agent vẫn là entity quyết định đọc gì, đọc lúc nào, đọc sâu tới đâu.

## What Changes

### Issue 1: Platform-only knowledge (skills + learned_skills + task_rules) không tới được CLI agent
- Thêm field `ContextFiles map[string]string` vào `engine.CodeStepRequest` (key = path tương đối trong `.autocode/context/`, value = nội dung file).
- `cli.go: RunCodeStep` ghi các file này ngay bên cạnh `.autocode/prompt.md` (cùng vòng đời tạo/dọn), và set `env["AUTOCODE_CONTEXT_DIR"]` trỏ container-side path — không cần lifecycle mới, tái dùng cleanup sẵn có (`rm -rf .autocode` trong script + `defer os.RemoveAll` phía host).
- Thêm `PromptAssembler.MaterializeCLIContext(ctx, task, agent, stepID) (map[string]string, error)` trong `internal/prompts` — build 2 layer:
  - **Layer 1 (always)**: `manifest.json` + `README.md` — mô tả context tồn tại + số lượng, không phụ thuộc relevance.
  - **Layer 2 (relevant, pre-resolved)**: `relevant/skills/<name>.md` (top-5 qua `resolveSkills()` đã có sẵn — fix đúng bug relevance-scoring của v1), `relevant/learned_skills.md`, `relevant/task_rules.md`.
  - **Layer 3 (catalog, đầy đủ nhưng chỉ index — KHÔNG nằm trong scope P0/P1 của spec này)**: index tên+mô tả toàn bộ skill/learned_skill, để agent tự quyết fetch thêm nếu cần — ghi chú làm hướng mở rộng, không implement ngay để giữ tối thiểu.
- 3 CLI steps gọi materializer, truyền `contextFiles` xuống `RunCLIStep`, và chỉ thêm **1 đoạn text ngắn** trỏ `$AUTOCODE_CONTEXT_DIR` vào instruction — không dump nội dung skill vào text.

## Capabilities

### New Capabilities
- `engine.CodeStepRequest.ContextFiles` — file map ghi vào `.autocode/context/` trước khi CLI chạy, dọn tự động cùng `.autocode/`.
- `PromptAssembler.MaterializeCLIContext()` — build manifest + relevant-context file map cho 1 task/step, dùng đúng `resolveSkills()` hiện có.
- `AUTOCODE_CONTEXT_DIR` env var — container-side path CLI có thể tự inspect (`ls`, `cat`).

### Modified Capabilities
- `engine.CodeStepRequest` — thêm field `ContextFiles`.
- `cli.go: RunCodeStep` — ghi context files, set env var.
- `steps.CLIStepRunner.RunCLIStep` — thêm tham số `contextFiles map[string]string`.
- `StepPromptLoader` interface — thêm method `MaterializeCLIContext`.
- `cli_analyze.go` / `cli_spec.go` / `cli_implement.go` — gọi materializer, instruction chỉ thêm 1 đoạn pointer ngắn.

### Removed Capabilities
- v1's full-text skill dump trong instruction string (chưa từng merge — bị thay thế hoàn toàn).
- v1.5's "copy toàn bộ `.data/projects/{id}/skills/`" không phân biệt relevant/không (thay bằng Layer 1 always + Layer 2 relevant).

## Impact

| Area | Files Affected |
|------|----------------|
| Engine request struct | `internal/orchestrator/engine/engine.go` |
| Engine run | `internal/orchestrator/engine/cli.go` |
| CLI step runner (interface + impl) | `internal/orchestrator/steps/services.go`, `internal/orchestrator/cli_spec_step.go` |
| Assembler | `internal/prompts/assembler.go` |
| CLI Steps | `internal/orchestrator/steps/cli_analyze.go`, `cli_spec.go`, `cli_implement.go` |
| Test Mocks | `cli_analyze_test.go`, `cli_spec_test.go`, `cli_implement_test.go`, `cli_spec_first_integration_test.go`, `internal/orchestrator/engine/cli_test.go` |
