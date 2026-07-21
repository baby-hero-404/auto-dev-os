# Proposal: CLI Spec-First Flow (analyze → openspec → implement → merge request)

## Why

Khi execution engine = `cli` (xem [`pluggable-execution-engine/`](../pluggable-execution-engine/proposal.md)), DAG hiện tại (context_load → analyze → plan → code → merge → review → fix → test → pr, `server/internal/workflow/step.go`) không còn phù hợp: phần lớn các step đó tồn tại để phục vụ tool-loop API-native mà server tự điều khiển. CLI agent (Claude Code, Codex…) đã tự có tool-loop, context loading, planning và self-review bên trong nó.

Học từ OpenSpec + ai-sdlc (`docs/references/README.md`): pipeline cho black-box agent nên là **spec-first** — server chỉ cần đảm bảo (1) agent hiểu đúng project, (2) có bản spec được duyệt làm hợp đồng, (3) implement bám spec, (4) kết quả ra merge request. Spec artifact đồng thời là thứ con người review được ở UI thay cho per-tool-call timeline đã mất khi outsource tool-loop.

## What Changes

### Issue 1: Pipeline mới cho CLI mode

- Thêm workflow definition `cli_spec_first` trong `server/internal/workflow/step.go`:
  `cli_analyze → cli_spec → cli_implement → cli_mr`
- `BuildWorkflow` chọn definition theo engine đã resolve của task (engine=cli → `cli_spec_first`; api_native → DAG hiện tại, không đổi).

### Issue 2: Các step mới (mỗi step = 1 lần spawn CLI với prompt chuyên biệt)

- **cli_analyze**: CLI được prompt phân tích repo + task description, output file `.autocode/analysis.md` (tech stack, files liên quan, risks). Server đọc file này lưu vào `task.Analysis`.
- **cli_spec**: CLI authoring OpenSpec set vào `docs/openspecs/<task-slug>/` trong worktree (4 files theo convention của chính Auto Code OS). Server parse `proposal.md` + `tasks.md` để hiển thị UI; gate approve (tùy autonomy setting của project) trước khi sang implement.
- **cli_implement**: CLI được prompt implement theo spec set, tick checkboxes trong `tasks.md` khi xong. Kết quả đánh giá bằng git diff (như REQ-005 của engine set).
- **cli_mr**: tái dùng PR step hiện có (`orchestrator/steps/pr.go`) — push branch + tạo merge request; spec set nằm trong diff nên reviewer thấy cả spec lẫn code.

### Issue 3: Spec artifacts trong UI

- Task detail: tab/panel "Spec" render proposal + tasks checkboxes (đọc từ worktree qua API có sẵn hoặc endpoint mới đọc file worktree).
- Khi autonomy = supervised: nút Approve/Request-changes trên spec trước khi `cli_implement` chạy.

## Capabilities

### New Capabilities
- Workflow definition thứ hai (`cli_spec_first`) chọn theo engine.
- Spec authoring tự động thành OpenSpec set trong repo của user.
- Spec approval gate (supervised mode).
- Spec panel trong task detail UI.

### Modified Capabilities
- `BuildWorkflow` nhận engine để chọn definition.
- PR step dùng được từ cả 2 flow.

### Removed Capabilities
- Không có (DAG API-native giữ nguyên).

## Impact

| Area | Files Affected |
|------|----------------|
| Workflow | `server/internal/workflow/step.go` |
| Steps (new) | `server/internal/orchestrator/steps/cli_analyze.go`, `cli_spec.go`, `cli_implement.go` |
| Prompts (new) | `server/internal/prompts/steps/cli_analyze.md`, `cli_spec.md`, `cli_implement.md` |
| Orchestrator | `worker.go` (definition selection), `steps/pr.go` (reuse) |
| API | endpoint đọc spec files từ worktree + approve action |
| Web | task detail: Spec panel + approve controls; timeline hiển thị 4 step mới |
