# Proposal: Definition-of-Ready Gate (P2.2)

## Why

Task thiếu spec (không acceptance criteria, không file scope) vẫn được dispatch thẳng vào coding — agent đoán mò, đốt token, kết quả lệch ý định. ai-sdlc (reference) chặn việc này bằng DoR gate: task chưa đủ "ready" thì không dispatch, operator phải trả lời câu hỏi mở trước. Auto Code OS đã có sẵn nền: `task.SpecStatus`, `task.Clarifications` (jsonb), `task.Analysis` — gate chỉ cần enforce.

## What Changes

### Issue 1: dor_check step

- Thêm step `dor_check` vào DAG giữa `analyze` và `plan`/`code` (`server/internal/workflow/step.go`).
- Step validate task có đủ: (a) acceptance criteria non-empty (từ analysis hoặc description có section), (b) file scope ước lượng (analysis đã produce), (c) không còn clarification nào `status=open`.
- Đủ → pass-through (nhanh, không gọi LLM). Thiếu → generate danh sách câu hỏi (1 LLM call nhỏ) ghi vào `task.Clarifications`, job pause trạng thái `awaiting_clarification`.

### Issue 2: Clarification resolution flow

- API answer clarification (nếu chưa có endpoint) → khi tất cả câu hỏi answered, job resume từ `dor_check` (re-validate rồi đi tiếp).
- UI: task detail hiển thị câu hỏi mở + form trả lời (khảo sát tái dùng UI clarifications nếu đã có).

### Issue 3: Bypass cho hotfix

- Label `hotfix` hoặc autonomy=`autonomous` → gate chỉ log warning, không chặn (theo nguyên tắc governance: gate chặt cho supervised, mềm cho autonomous).

## Capabilities

### New Capabilities
- Gate `dor_check` trong DAG với pause/resume.
- Sinh câu hỏi làm rõ tự động khi task thiếu thông tin.

### Modified Capabilities
- DAG có thêm 1 node; `plan`/`code` dependsOn `dor_check`.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Workflow | `server/internal/workflow/step.go` |
| Steps (new) | `server/internal/orchestrator/steps/dor_check.go` |
| Prompts (new) | `server/internal/prompts/steps/dor_check.md` |
| API/UI | clarification answer endpoint + task detail panel |
